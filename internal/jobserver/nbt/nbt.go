// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package nbt

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/DataDog/orchestrion/internal/files"
	"github.com/DataDog/orchestrion/internal/jobserver/common"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

const (
	subjectPrefix = "never-build-twice."

	startSubject  = subjectPrefix + "start"
	finishSubject = subjectPrefix + "finish"
)

type (
	service struct {
		// state is a map of import path to a [sync.Map] of build ID to buildState.
		state sync.Map
		dir   string
	}
	buildState struct {
		initOnce sync.Once
		token    string          // Finalization token
		onDone   func()          // Called once the original task has completed
		done     <-chan struct{} // Blocks until the original task has completed

		isDone atomic.Bool      // Whether the original task has completed yet.
		files  map[Label]string // Additional files produced by the original task. Available once done.
		error  error            // Error from the original task. Available once done.
	}
)

func Subscribe(ctx context.Context, conn *nats.Conn) (cleanup func(context.Context) error, resErr error) {
	dir, err := os.MkdirTemp("", "orchestrion.nbt-*")
	if err != nil {
		return nil, fmt.Errorf("creating storage directory: %w", err)
	}
	defer func() {
		if resErr == nil {
			return
		}
		if err := os.RemoveAll(dir); err != nil {
			resErr = errors.Join(resErr, err)
		}
	}()

	s := &service{dir: dir}
	_, err = conn.Subscribe(startSubject,
		common.HandleRequest(
			zerolog.Ctx(ctx).With().Str("nats.subject", startSubject).Logger().WithContext(ctx),
			s.start,
		),
	)
	if err != nil {
		return nil, err
	}

	_, err = conn.Subscribe(finishSubject,
		common.HandleRequest(
			zerolog.Ctx(ctx).With().Str("nats.subject", finishSubject).Logger().WithContext(ctx),
			s.finish,
		),
	)
	if err != nil {
		return nil, err
	}

	cleanup = func(context.Context) error { return os.RemoveAll(dir) }
	return cleanup, nil
}

type (
	// StartRequest informs the job server that the caller is starting a new
	// compilation task for the specified [StartRequest.ImportPath].
	StartRequest struct {
		ImportPath string `json:"importPath"`
		BuildID    string `json:"buildID"`
	}
	// StartResponse informs the caller about what should be done with the
	// compilation task. If a [*StartResponse.FinishToken] is present, the caller
	// must proceed with the task, then send a [FinishRequest] using that token.
	// If a [*StartResponse.ArchivePath] is present, the caller must skip the task
	// and instead re-use the file at the specified path.
	StartResponse struct {
		// FinishToken is the token to be forwarded to the [FinishRequest] to inform
		// the job server about the outcome of the build. It cannot be blank unless
		// [*StartResponse.ArchivePath] is not blank.
		FinishToken string `json:"token,omitempty"`
		// Files is the set of files produced by the original task, by label. These
		// files must be re-used instead of re-created. Always blank if
		// [*StartResponse.FinishToken] is not blank.
		Files map[Label]string `json:"extra,omitempty"`
	}
	// Label is a label identifying an additional object produced by a task. It
	// must be a valid part of a file name.
	Label string
)

const (
	LabelArchive Label = "_pkg_.a"
	LabelAsmhdr  Label = "go_asm.h"
)

func (StartRequest) Subject() string           { return startSubject }
func (StartRequest) ResponseIs(*StartResponse) {}

func (s *service) start(ctx context.Context, req StartRequest) (*StartResponse, error) {
	if req.ImportPath == "" || req.BuildID == "" {
		return nil, fmt.Errorf("invalid request: %#v", req)
	}

	rawPkgMap, _ := s.state.LoadOrStore(req.ImportPath, &sync.Map{})
	pkgMap, _ := rawPkgMap.(*sync.Map)

	rawState, reused := pkgMap.LoadOrStore(req.ImportPath, &buildState{})
	state, _ := rawState.(*buildState)

	// Initialize the build state.
	state.initOnce.Do(func() {
		state.token = uuid.NewString()
		// We use a cancellable context as a barrier here...
		ctx, isDone := context.WithCancel(ctx)
		state.done = ctx.Done()
		state.onDone = isDone
	})

	// If the build state is re-used, wait for the original to complete...
	if reused {
		<-state.done
		if state.error != nil {
			return nil, state.error
		}

		if len(state.files) == 0 {
			// The context has expired or the upstream context has been canceled.
			// We'll return this as [context.Canceled] either way.
			return nil, context.Canceled
		}

		return &StartResponse{Files: state.files}, nil
	}

	// Otherwise, return a finalization token, etc...
	zerolog.Ctx(ctx).Debug().Str("token", state.token).Str("import-path", req.ImportPath).Msg("Compile task started")
	return &StartResponse{FinishToken: state.token}, nil
}

type (
	// FinishRequest informs the job server about the result of a compilation
	// task.
	FinishRequest struct {
		// ImportPath is the import path of the package that was built.
		ImportPath string `json:"importPath"`
		// BuildID is the build ID for the package being built.
		BuildID string `json:"buildID"`
		// FinishToken is forwarded from [*StartResponse.FinishToken], and cannot be
		// blank.
		FinishToken string `json:"token"`
		// Files is a list of files produced by the compilation task, associated to
		// a user-defined label.
		Files map[Label]string `json:"extra,omitempty"`
		// Error is the error that occurred as a result of this compilation task, if
		// any.
		Error *string `json:"error,omitempty"`
	}
	FinishResponse struct {
		/* unused */
	}
)

func (FinishRequest) Subject() string            { return finishSubject }
func (FinishRequest) ResponseIs(*FinishResponse) {}

var errNoFilesNorError = errors.New("missing files, and no error reported")

func (s *service) finish(ctx context.Context, req FinishRequest) (*FinishResponse, error) {
	if req.ImportPath == "" || req.FinishToken == "" {
		return nil, fmt.Errorf("invalid request: %#v", req)
	}

	rawPkgMap, found := s.state.Load(req.ImportPath)
	if !found {
		return nil, fmt.Errorf("no build started for %q", req.ImportPath)
	}
	pkgMap, _ := rawPkgMap.(*sync.Map)

	rawState, found := pkgMap.Load(req.ImportPath)
	if !found {
		return nil, fmt.Errorf("no build started for %q with build ID %q", req.ImportPath, req.BuildID)
	}

	log := zerolog.Ctx(ctx).With().
		Str("import-path", req.ImportPath).
		Logger()
	ctx = log.WithContext(ctx)

	state, _ := rawState.(*buildState)
	if state.token != req.FinishToken {
		log.Warn().
			Str("expected", state.token).
			Str("actual", req.FinishToken).
			Msg("Invalid finish token")
		return nil, fmt.Errorf("invalid finish token for %q: %q", req.ImportPath, req.FinishToken)
	}

	if !state.isDone.CompareAndSwap(false, true) {
		log.Info().Msg("Task was already completed (concurrent retry?)")
		return &FinishResponse{}, nil
	}

	defer state.onDone()
	log.Debug().
		Any("files", req.Files).
		Any("error", req.Error).
		Msg("Compile task finished")

	if req.Error != nil {
		state.error = errors.New(*req.Error)
		return &FinishResponse{}, nil
	}

	if len(req.Files) == 0 {
		state.error = errNoFilesNorError
		return nil, state.error
	}

	dir := filepath.Join(s.dir, uuid.NewSHA1(ns, []byte(req.ImportPath)).String(), uuid.NewSHA1(ns, []byte(req.BuildID)).String())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		state.error = fmt.Errorf("creating storage directory: %w", err)
		return nil, state.error
	}

	state.files = make(map[Label]string, len(req.Files))
	for label, path := range req.Files {
		filename := filepath.Join(dir, string(label))
		if err := files.Copy(ctx, path, filename); err != nil {
			state.files = nil
			state.error = fmt.Errorf("persisting additional file %q (%q): %w", path, label, err)
			return nil, state.error
		}
		state.files[label] = filename
	}

	return &FinishResponse{}, nil
}

// ns is an arbitrary UUID used as a namespace for hashing import paths when storing artifacts in
// the temporary storage location.
var ns = uuid.MustParse("4BFB6F4B-212C-43A0-A581-A29C8B3D3BE4")

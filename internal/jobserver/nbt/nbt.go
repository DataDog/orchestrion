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
		state sync.Map
		dir   string
	}
	buildState struct {
		initOnce sync.Once
		buildID  string
		token    string          // Finalization token
		onDone   func()          // Called once the original task has completed
		done     <-chan struct{} // Blocks until the original task has completed

		isDone  atomic.Bool // Whether the original task has completed yet.
		archive string      // Path to the archive produced by the original task. Available once done.
		error   error       // Error from the original task. Available once done.
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
		// ArchivePath is the path of the archive produced for the same
		// [StartRequest.ImportPath] by another task. That archive must be re-used
		// instead of creating a new one. It cannot be blank unless
		// [*StartResponse.FinishToken] is not blank.
		ArchivePath string `json:"archivePath,omitempty"`
	}
)

func (StartRequest) Subject() string             { return startSubject }
func (*StartResponse) IsResponseTo(StartRequest) {}

func (s *service) start(ctx context.Context, req StartRequest) (*StartResponse, error) {
	if req.ImportPath == "" || req.BuildID == "" {
		return nil, fmt.Errorf("invalid request: %#v", req)
	}

	rawState, reused := s.state.LoadOrStore(req.ImportPath, &buildState{buildID: req.BuildID})
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
		if state.buildID != req.BuildID {
			return nil, fmt.Errorf("mismatched build ID for %q: %q != %q", req.ImportPath, state.buildID, req.BuildID)
		}

		<-state.done
		if state.error != nil {
			return nil, state.error
		}
		return &StartResponse{ArchivePath: state.archive}, nil
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
		// FinishToken is forwarded from [*StartResponse.FinishToken], and cannot be
		// blank.
		FinishToken string `json:"token"`
		// ArchivePath is the path to the archive produced by the compilation task.
		// It must not be blank unless [FinishRequest.Error] is not nil. If present,
		// the file must exist on disk, and will be hard-linked (or copied) into a
		// temporary directory so it can be re-used by subsequent [StartRequest] for
		// the same [FinishRequest.ImportPath].
		ArchivePath string `json:"archivePath,omitempty"`
		// Error is the error that occurred as a result of this compilation task, if
		// any.
		Error *string `json:"error,omitempty"`
	}
	FinishResponse struct {
		/* unused */
	}
)

func (FinishRequest) Subject() string              { return finishSubject }
func (*FinishResponse) IsResponseTo(FinishRequest) {}

var errNoArchiveNorError = errors.New("missing archive path, and no error reported")

func (s *service) finish(ctx context.Context, req FinishRequest) (*FinishResponse, error) {
	if req.ImportPath == "" || req.FinishToken == "" {
		return nil, fmt.Errorf("invalid request: %#v", req)
	}

	rawState, found := s.state.Load(req.ImportPath)
	if !found {
		return nil, fmt.Errorf("no build started for %q", req.ImportPath)
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
		Str("archive-path", req.ArchivePath).
		Any("error", req.Error).
		Msg("Compile task finished")

	if req.Error != nil {
		state.error = errors.New(*req.Error)
	} else if req.ArchivePath == "" {
		state.error = errNoArchiveNorError
		return nil, state.error
	} else if file, err := s.persist(ctx, req.ImportPath, req.ArchivePath); err != nil {
		state.error = fmt.Errorf("persisting archive %q: %w", req.ArchivePath, err)
		return nil, state.error
	} else {
		state.archive = file
	}

	return &FinishResponse{}, nil
}

var ns = uuid.MustParse("4BFB6F4B-212C-43A0-A581-A29C8B3D3BE4")

func (s *service) persist(ctx context.Context, name string, path string) (string, error) {
	guid := uuid.NewSHA1(ns, []byte(name)).String()
	res := filepath.Join(s.dir, guid)

	if err := files.Copy(ctx, path, res); err != nil {
		return "", err
	}
	return res, nil
}

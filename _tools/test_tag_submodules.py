# Unless explicitly stated otherwise all files in this repository are licensed
# under the Apache License Version 2.0.
# This product includes software developed at Datadog (https://www.datadoghq.com/).
# Copyright 2026-present Datadog, Inc.

"""Exercise release tagging against local Git repositories, never GitHub."""

import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("tag-submodules.sh")
TAG = "v1.13.1"
MOCK_GH = r'''#!/usr/bin/env python3
import json
import os
import subprocess
import sys

args = sys.argv[1:]
if args[:2] == ["release", "view"]:
    if os.environ.get("TEST_RELEASE_STATE") == "missing":
        sys.exit(1)
    print("" if os.environ.get("TEST_RELEASE_STATE") == "draft" else args[2])
    sys.exit(0)

fields = dict(arg.split("=", 1) for arg in args if "=" in arg)
endpoint = args[3]
with open(os.environ["TEST_API_LOG"], "a") as log:
    log.write(json.dumps({"endpoint": endpoint, "fields": fields}) + "\n")
git = ["git", "--git-dir", os.environ["TEST_REMOTE"]]
if endpoint == "repos/DataDog/orchestrion/git/tags":
    content = (f"object {fields['object']}\ntype commit\ntag {fields['tag']}\n"
               "tagger Release Test <test@example.com> 1700000000 +0000\n\n"
               f"{fields['message']}\n")
    print(subprocess.check_output(git + ["mktag"], input=content, text=True).strip())
elif endpoint == "repos/DataDog/orchestrion/git/refs":
    outcome = os.environ.get("TEST_REF_OUTCOME", "success")
    if outcome == "reject":
        print("GH013: Cannot create ref due to creations being restricted", file=sys.stderr)
        sys.exit(1)
    subprocess.run(git + ["update-ref", fields["ref"], fields["sha"], "0" * 40], check=True)
    if outcome == "lost-response":
        print("connection lost after ref creation", file=sys.stderr)
        sys.exit(1)
else:
    raise AssertionError(f"Unexpected GitHub operation: {args}")
'''


MOCK_GIT = r'''#!/usr/bin/env python3
import os
from pathlib import Path
import sys

args = sys.argv[1:]
if args[0] == "fetch" and os.environ.get("TEST_FAIL_FETCH"):
    print("fetch failed", file=sys.stderr)
    sys.exit(1)
if args[0] == "ls-remote" and os.environ.get("TEST_FAIL_PREFLIGHT"):
    counter = Path(os.environ["TEST_READ_COUNT"])
    count = int(counter.read_text()) + 1 if counter.exists() else 1
    counter.write_text(str(count))
    if count > 1:
        print("remote read failed", file=sys.stderr)
        sys.exit(1)
os.execv(os.environ["TEST_REAL_GIT"], ["git", *args])
'''


class TagSubmodulesTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.repo = self.root / "repo"
        self.remote = self.root / "remote.git"
        self.repo.mkdir()
        self.api_log = self.root / "api.jsonl"
        git_executable = shutil.which("git")
        if git_executable is None:
            raise RuntimeError("Git is required for release-tagging tests")
        self.env = os.environ.copy()
        self.env.update({
            "GIT_CONFIG_GLOBAL": os.devnull,
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_AUTHOR_NAME": "Release Test",
            "GIT_AUTHOR_EMAIL": "test@example.com",
            "GIT_COMMITTER_NAME": "Release Test",
            "GIT_COMMITTER_EMAIL": "test@example.com",
            "GH_TOKEN": "local-test-only",
            "TEST_REMOTE": str(self.remote),
            "TEST_API_LOG": str(self.api_log),
            "TEST_REAL_GIT": git_executable,
            "TEST_READ_COUNT": str(self.root / "reads"),
        })
        self.git("init", "--bare", "--template=", str(self.remote))
        self.git("init", "--template=", "-b", "main")
        modules = {
            "go.mod": "github.com/DataDog/orchestrion",
            "instrument/go.mod": "github.com/DataDog/orchestrion/instrument",
            "extra/go.mod": '"github.com/DataDog/orchestrion/extra"',
            "samples/go.mod": "github.com/DataDog/orchestrion/_samples",
            "_tools/go.mod": "github.com/DataDog/orchestrion/_tools",
            "internal/testdata/fixture/go.mod": "github.com/DataDog/orchestrion/internal/testdata/fixture",
            "thirdparty/go.mod": "example.com/thirdparty",
        }
        for name, module in modules.items():
            path = self.repo / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(f"module {module}\n\ngo 1.25.0\n")
        self.git("add", ".")
        self.git("-c", "commit.gpgsign=false", "commit", "-m", "release fixture")
        self.sha = self.git("rev-parse", "HEAD")
        self.git("tag", TAG)
        self.git("remote", "add", "origin", str(self.remote))
        self.git("push", "origin", "main", f"refs/tags/{TAG}")
        bin_dir = self.root / "bin"
        bin_dir.mkdir()
        gh = bin_dir / "gh"
        gh.write_text(MOCK_GH)
        gh.chmod(0o755)
        git = bin_dir / "git"
        git.write_text(MOCK_GIT)
        git.chmod(0o755)
        self.env["PATH"] = str(bin_dir) + os.pathsep + self.env["PATH"]

    def git(self, *args):
        return subprocess.check_output(
            ["git", *args], cwd=self.repo, env=self.env, text=True,
            stderr=subprocess.PIPE,
        ).strip()

    def remote_ref(self, tag, peel=False):
        ref = f"refs/tags/{tag}" + ("^{commit}" if peel else "")
        result = subprocess.run(
            ["git", "--git-dir", str(self.remote), "rev-parse", "--verify", ref],
            env=self.env, text=True, capture_output=True,
        )
        return result.stdout.strip() if result.returncode == 0 else None

    def run_tagger(self, tag=TAG, sha=None, **env):
        return subprocess.run(
            ["bash", str(SCRIPT), tag, sha or self.sha], cwd=self.repo,
            env={**self.env, **env}, text=True, capture_output=True,
        )

    def resolve(self, **env):
        return subprocess.run(
            ["bash", "-c", 'source "$1"; sha=$(resolve_release "$2"); printf "%s\\n" "$sha"',
             "test", str(SCRIPT), TAG],
            cwd=self.repo, env={**self.env, **env}, text=True, capture_output=True,
        )

    def assert_success(self, result):
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def publish_existing(self, tag, commit=None, annotated=False):
        args = ["tag"] + (["-a", "-m", "existing release"] if annotated else [])
        self.git(*args, tag, commit or self.sha)
        self.git("push", "origin", f"refs/tags/{tag}")

    def test_creates_only_matching_modules_at_release_commit(self):
        self.assert_success(self.run_tagger())
        for name in ["instrument", "extra"]:
            self.assertEqual(self.remote_ref(f"{name}/{TAG}", peel=True), self.sha)
            self.assertNotEqual(self.remote_ref(f"{name}/{TAG}"), self.sha)  # Annotated.
        tags = self.git("ls-remote", "--tags", "origin")
        self.assertNotIn("samples/", tags)
        self.assertNotIn("_tools/", tags)
        self.assertNotIn("testdata/", tags)
        self.assertNotIn("thirdparty/", tags)

    def test_retry_preserves_existing_tag_objects(self):
        self.assert_success(self.run_tagger())
        before = self.git("ls-remote", "--tags", "origin")
        api_before = self.api_log.read_text()
        self.assert_success(self.run_tagger())
        self.assertEqual(self.git("ls-remote", "--tags", "origin"), before)
        self.assertEqual(self.api_log.read_text(), api_before)

    def test_partial_release_preserves_lightweight_tag(self):
        self.publish_existing(f"instrument/{TAG}")
        self.assert_success(self.run_tagger())
        self.assertEqual(self.remote_ref(f"instrument/{TAG}"), self.sha)
        self.assertEqual(self.remote_ref(f"extra/{TAG}", peel=True), self.sha)

    def test_conflict_fails_before_creating_any_missing_tags(self):
        self.git("-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "different commit")
        other = self.git("rev-parse", "HEAD")
        self.publish_existing(f"instrument/{TAG}", other, annotated=True)
        result = self.run_tagger()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("different commit", result.stderr)
        self.assertIsNone(self.remote_ref(f"extra/{TAG}"))
        self.assertFalse(self.api_log.exists())

    def test_uses_release_tree_instead_of_current_checkout(self):
        path = self.repo / "later/go.mod"
        path.parent.mkdir()
        path.write_text("module github.com/DataDog/orchestrion/later\n")
        self.git("add", ".")
        self.git("-c", "commit.gpgsign=false", "commit", "-m", "unreleased module")
        self.assert_success(self.run_tagger())
        self.assertIsNone(self.remote_ref(f"later/{TAG}"))

    def test_root_tag_must_match_expected_commit(self):
        self.git("-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "later")
        result = self.run_tagger(sha=self.git("rev-parse", "HEAD"))
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.api_log.exists())

    def test_invalid_or_missing_root_tag_cannot_create_tags(self):
        for tag in ["main", "instrument/v1.13.1", "v1.2.3;echo", "v01.2.3", "v9.9.9"]:
            with self.subTest(tag=tag):
                self.assertNotEqual(self.run_tagger(tag=tag).returncode, 0)
        self.assertFalse(self.api_log.exists())

    def test_reconciles_a_lost_success_response(self):
        self.assert_success(self.run_tagger(TEST_REF_OUTCOME="lost-response"))
        self.assertEqual(self.remote_ref(f"instrument/{TAG}", peel=True), self.sha)

    def test_permission_failure_is_not_treated_as_success(self):
        result = self.run_tagger(TEST_REF_OUTCOME="reject")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("GH013", result.stderr)
        self.assertIsNone(self.remote_ref(f"extra/{TAG}"))

    def test_resolves_published_release_from_root_tag(self):
        result = self.resolve()
        self.assert_success(result)
        self.assertEqual(result.stdout.strip(), self.sha)

    def test_failed_preflight_read_cannot_create_tags(self):
        result = self.run_tagger(TEST_FAIL_PREFLIGHT="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.api_log.exists())

    def test_failed_fetch_cannot_reuse_stale_fetch_head(self):
        self.git("fetch", "origin", f"refs/tags/{TAG}")
        self.assertNotEqual(self.resolve(TEST_FAIL_FETCH="1").returncode, 0)
        self.assertFalse(self.api_log.exists())

    def test_draft_or_missing_release_cannot_be_repaired(self):
        for state in ["draft", "missing"]:
            with self.subTest(state=state):
                self.assertNotEqual(self.resolve(TEST_RELEASE_STATE=state).returncode, 0)
        self.assertFalse(self.api_log.exists())


class TrustPolicyTest(unittest.TestCase):
    def policy_matches(self, policy, event, ref, repository="DataDog/orchestrion", workflow="release.yml"):
        path = SCRIPT.parent.parent / ".github/chainguard" / f"self.github.release.{policy}.sts.yaml"
        text = path.read_text()
        # These policies use only plain scalar values; yamlfmt checks YAML syntax separately.
        subject = f"repo:{repository}:ref:{ref}"
        claims = {
            "event_name": event,
            "ref": ref,
            "repository": repository,
            "workflow_ref": f"{repository}/.github/workflows/{workflow}@{ref}",
        }
        values = dict(line.strip().split(": ", 1) for line in text.splitlines()
                      if ": " in line and not line.startswith("#"))
        self.assertEqual(values["issuer"], "https://token.actions.githubusercontent.com")
        self.assertEqual(text.split("permissions:\n")[1].strip(), "contents: write")
        self.assertNotIn("job_workflow_ref", values)
        if "subject" in values and values["subject"] != subject:
            return False
        if "subject_pattern" in values and not re.fullmatch(values["subject_pattern"], subject):
            return False
        return all(re.fullmatch(values[key], value) for key, value in claims.items())

    def test_published_policy_allows_only_release_workflow_on_version_tags(self):
        for tag in ["v1.13.1", "v1.14.0-rc.1"]:
            self.assertTrue(self.policy_matches("published", "release", f"refs/tags/{tag}"))
        for event, ref, repo, workflow in [
            ("pull_request", "refs/pull/1/merge", "DataDog/orchestrion", "release.yml"),
            ("workflow_dispatch", "refs/tags/v1.13.1", "DataDog/orchestrion", "release.yml"),
            ("release", "refs/heads/main", "DataDog/orchestrion", "release.yml"),
            ("release", "refs/tags/instrument/v1.13.1", "DataDog/orchestrion", "release.yml"),
            ("release", "refs/tags/v1.13.1", "fork/orchestrion", "release.yml"),
            ("release", "refs/tags/v1.13.1", "DataDog/orchestrion", "other.yml"),
        ]:
            self.assertFalse(self.policy_matches("published", event, ref, repo, workflow))

    def test_manual_policy_allows_only_dispatch_on_main(self):
        self.assertTrue(self.policy_matches("manual", "workflow_dispatch", "refs/heads/main"))
        for event, ref in [("release", "refs/heads/main"),
                           ("pull_request", "refs/pull/1/merge"),
                           ("workflow_dispatch", "refs/heads/feature"),
                           ("workflow_dispatch", "refs/tags/v1.13.1")]:
            self.assertFalse(self.policy_matches("manual", event, ref))
        self.assertFalse(self.policy_matches("manual", "workflow_dispatch", "refs/heads/main", "fork/orchestrion"))
        self.assertFalse(self.policy_matches("manual", "workflow_dispatch", "refs/heads/main", workflow="other.yml"))


if __name__ == "__main__":
    unittest.main()

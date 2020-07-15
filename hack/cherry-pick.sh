#!/usr/bin/env bash

# Copyright 2015 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Usage Instructions: https://git.k8s.io/community/contributors/devel/sig-release/cherry-picks.md

# Checkout a PR from GitHub. (Yes, this is sitting in a Git tree. How
# meta.) Assumes you care about pulls from remote "upstream" and
# checks them out to a branch named:
#  automated-cherry-pick-of-<pr>-<target branch>-<timestamp>

# Modified by Mattermost to do a real cherry-pick instead of applying patches
# via git am

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
declare -r REPO_ROOT
cd "${REPO_ROOT}"

STARTINGBRANCH=$(git symbolic-ref --short HEAD)
declare -r STARTINGBRANCH
declare -r REBASEMAGIC="${REPO_ROOT}/.git/rebase-apply"
DRY_RUN=${DRY_RUN:-""}
REGENERATE_DOCS=${REGENERATE_DOCS:-""}
UPSTREAM_REMOTE=${UPSTREAM_REMOTE:-upstream}
FORK_REMOTE=${FORK_REMOTE:-origin}
MAIN_REPO_ORG=${MAIN_REPO_ORG:-$(git remote get-url "$UPSTREAM_REMOTE" | awk '{gsub(/http[s]:\/\/|git@/,"")}1' | awk -F'[@:./]' 'NR==1{print $3}')}
MAIN_REPO_NAME=${MAIN_REPO_NAME:-$(git remote get-url "$UPSTREAM_REMOTE" | awk '{gsub(/http[s]:\/\/|git@/,"")}1' | awk -F'[@:./]' 'NR==1{print $4}')}

if [[ -z ${GITHUB_USER:-} ]]; then
  echo "Please export GITHUB_USER=<your-user> (or GH organization, if that's where your fork lives)"
  exit 1
fi

if ! which hub > /dev/null; then
  echo "Can't find 'hub' tool in PATH, please install from https://github.com/github/hub"
  exit 1
fi

if [[ "$#" -lt 2 ]]; then
  echo "${0} <remote branch> <pr-number> <commit-sha>: cherry pick a <commit> from <pr> onto <remote branch> and leave instructions for proposing pull request"
  echo
  echo "  Checks out <remote branch> and handles the cherry-pick of <commits> for you."
  echo "  Examples:"
  echo "    $0 upstream/release-3.14 12345 df243fd # Cherry-picks commit sha df243fd onto upstream/release-3.14 and proposes that as a PR with a commit message referring to PR 12345"
  echo
  echo "  Set the DRY_RUN environment var to skip git push and creating PR."
  echo "  This is useful for creating patches to a release branch without making a PR."
  echo "  When DRY_RUN is set the script will leave you in a branch containing the commits you cherry-picked."
  echo
  echo " Set UPSTREAM_REMOTE (default: upstream) and FORK_REMOTE (default: origin)"
  echo " To override the default remote names to what you have locally."
  exit 2
fi

if git_status=$(git status --porcelain --untracked=no 2>/dev/null) && [[ -n "${git_status}" ]]; then
  echo "!!! Dirty tree. Clean up and try again."
  exit 1
fi

while unmerged=$(git status --porcelain | grep ^U) && [[ -n ${unmerged} ]] || [[ -e "${REBASEMAGIC}" ]]; do
  echo
  echo "!!! 'git cherry-pick' or other operation in progress. Clean up and try again."
  exit 1
done

declare -r BRANCH="$1"
declare -r PULL="$2"
declare -r COMMITSHA="$3"

echo "+++ Updating remotes..."
git remote update "${UPSTREAM_REMOTE}" "${FORK_REMOTE}"
echo "+++ Updating remotes done..."
if ! git log -n1 --format=%H "${BRANCH}" >/dev/null 2>&1; then
  echo "!!! '${BRANCH}' not found. The second argument should be something like ${UPSTREAM_REMOTE}/release-0.21."
  echo "    (In particular, it needs to be a valid, existing remote branch that I can 'git checkout'.)"
  exit 1
fi

NEWBRANCHREQ="automated-cherry-pick-of-#${PULL}" # "Required" portion for tools.
declare -r NEWBRANCHREQ
NEWBRANCH="$(echo "${NEWBRANCHREQ}-${BRANCH}" | sed 's/\//-/g')"
declare -r NEWBRANCH
NEWBRANCHUNIQ="${NEWBRANCH}-$(date +%s)"
declare -r NEWBRANCHUNIQ
echo "+++ Creating local branch ${NEWBRANCHUNIQ}"

cleanbranch=""
prtext=""
gitcleanup=false
function do_cleanup {
  if [[ "${gitcleanup}" == "true" ]]; then
    echo
    echo "+++ Aborting in-progress git cherry-pick."
    git cherry-pick --abort >/dev/null 2>&1 || true
  fi

  # return to the starting branch and delete the PR text file
  if [[ -z "${DRY_RUN}" ]]; then
    echo
    echo "+++ Returning you to the ${STARTINGBRANCH} branch and cleaning up."
    git checkout -f "${STARTINGBRANCH}" >/dev/null 2>&1 || true
    if [[ -n "${cleanbranch}" ]]; then
      git branch -D "${cleanbranch}" >/dev/null 2>&1 || true
    fi
    if [[ -n "${prtext}" ]]; then
      rm "${prtext}"
    fi
  fi
}
trap do_cleanup EXIT

function make-a-pr() {
  local rel
  rel="$(basename "${BRANCH}")"
  echo
  echo "+++ Creating a pull request on GitHub at ${GITHUB_USER}:${NEWBRANCH}"

  # This looks like an unnecessary use of a tmpfile, but it avoids
  # https://github.com/github/hub/issues/976 Otherwise stdin is stolen
  # when we shove the heredoc at hub directly, tickling the ioctl
  # crash.
  prtext="$(mktemp -t prtext.XXXX)" # cleaned in do_cleanup
  cat >"${prtext}" <<EOF
Automated cherry pick of #${PULL}

Cherry pick of #${PULL} on ${rel}.

/cc  @${ORIGINAL_AUTHOR}
EOF

hub pull-request -F "${prtext}" -h "${GITHUB_USER}:${NEWBRANCH}" -b "${MAIN_REPO_ORG}:${rel}"
}

git checkout -b "${NEWBRANCHUNIQ}" "${BRANCH}"
cleanbranch="${NEWBRANCHUNIQ}"

gitcleanup=true
echo
echo "+++ About to attempt cherry pick of PR #${PULL} with merge commit ${COMMITSHA}."
echo
git cherry-pick -x "${COMMITSHA}" || {
  while unmerged=$(git status --porcelain | grep ^U) && [[ -n ${unmerged} ]] || [[ -e "${REBASEMAGIC}" ]]; do
    echo
    echo "+++ Conflicts detected:"
    echo
    (git status --porcelain | grep ^U) || echo "!!! None. Did you git cherry-pick --continue or git --cherry-pick --abort?"
    echo "Aborting." >&2
    exit 1
  done

  echo "!!! git cherry-pick failed"
  exit 1
}
gitcleanup=false

if [[ -n "${DRY_RUN}" ]]; then
  echo "!!! Skipping git push and PR creation because you set DRY_RUN."
  echo "To return to the branch you were in when you invoked this script:"
  echo
  echo "  git checkout ${STARTINGBRANCH}"
  echo
  echo "To delete this branch:"
  echo
  echo "  git branch -D ${NEWBRANCHUNIQ}"
  exit 0
fi

if git remote -v | grep ^"${FORK_REMOTE}" | grep "${MAIN_REPO_ORG}/${MAIN_REPO_NAME}.git"; then
  echo "!!! You have ${FORK_REMOTE} configured as your ${MAIN_REPO_ORG}/${MAIN_REPO_NAME}.git"
  echo "This isn't normal. Leaving you with push instructions:"
  echo
  echo "+++ First manually push the branch this script created:"
  echo
  echo "  git push REMOTE ${NEWBRANCHUNIQ}:${NEWBRANCH}"
  echo
  echo "where REMOTE is your personal fork (maybe ${UPSTREAM_REMOTE}? Consider swapping those.)."
  echo "OR consider setting UPSTREAM_REMOTE and FORK_REMOTE to different values."
  echo
  make-a-pr
  cleanbranch=""
  exit 0
fi


echo
echo "+++ I'm about to do the following to push to GitHub (and I'm assuming ${FORK_REMOTE} is your personal fork):"
echo
echo "  git push ${FORK_REMOTE} ${NEWBRANCHUNIQ}:${NEWBRANCH}"
echo


git push "${FORK_REMOTE}" -f "${NEWBRANCHUNIQ}:${NEWBRANCH}"
make-a-pr
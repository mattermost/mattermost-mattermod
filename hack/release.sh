#!/usr/bin/env bash

set -o errexit
set -o xtrace
set -o pipefail

CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

if [ -z "${CURRENT_BRANCH}" -o "${CURRENT_BRANCH}" != "master" ]; then
    echo "Error: The current branch is '${CURRENT_BRANCH}', switch to 'master' to do the release."
    exit 1
fi

if [ -n "$(git status --short)" ]; then
    echo "Error: There are untracked/modified changes, commit or discard them before the release."
    exit 1
fi

RELEASE_VERSION=$1
CURRENT_VERSION=$2
FROM_MAKEFILE=$3

if [ -z "${RELEASE_VERSION}" ]; then
    if [ -z "${FROM_MAKEFILE}" ]; then
        echo "Error: VERSION is missing. e.g. ./release.sh <VERSION>"
    else
        echo "Error: missing value for 'version'. e.g. 'make release VERSION=x.y.z'"
    fi
    exit 1
fi

if [ -z "${CURRENT_VERSION}" ]; then
    CURRENT_VERSION=$(git describe --tags --exact-match 2>/dev/null || git describe --tags 2>/dev/null || echo "v0.0.1-$(COMMIT_HASH)")
fi

if [ "v${RELEASE_VERSION}" = "${CURRENT_VERSION}" ]; then
    echo "Error: provided version (v${RELEASE_VERSION}) already exists."
    exit 1
fi

if [ $(git describe --tags "v${RELEASE_VERSION}" 2>/dev/null) ]; then
    echo "Error: provided version (v${RELEASE_VERSION}) already exists."
    exit 1
fi

PWD=$(cd $(dirname "$0") && pwd -P)

# get closest GA tag, ignore alpha, beta and rc tags
function getClosestVersion() {
    for t in $(git tag --sort=-creatordate); do
        tag="$t"
        if [[ $tag == *"-alpha"* || $tag == *"-beta"* || $tag == *"-rc"* ]]; then
            continue
        fi
        break
    done
    echo "$tag" | sed 's/^v//'
}
CLOSEST_VERSION=$(getClosestVersion)

# Bump the released version
sed -i -E 's|'${CLOSEST_VERSION}'|'${RELEASE_VERSION}'|g' deploy/overlays/prod/kustomization.yaml
# On FreeBSD sed versions, -i takes an argument, so it creates a backup file with the next argument, which happens to be -E in this case. Hence, to keep things simple, we just delete the backup file.
# Just -i does not work on MacOS, and -i '' does not work on GNU. Hence we are stuck with this quirk.
rm -f deploy/overlays/prod/kustomization.yaml-E

# Commit changes
printf "\033[36m==> %s\033[0m\n" "Commit changes for release version v${RELEASE_VERSION}"
git add deploy/overlays/prod/kustomization.yaml
git commit -m "Release version v${RELEASE_VERSION}"

printf "\033[36m==> %s\033[0m\n" "Push commits for v${RELEASE_VERSION}"
git push origin master

# Tag the release
printf "\033[36m==> %s\033[0m\n" "Tag release v${RELEASE_VERSION}"
git tag --annotate --message "v${RELEASE_VERSION} Release" "v${RELEASE_VERSION}"

printf "\033[36m==> %s\033[0m\n" "Push tag release v${RELEASE_VERSION}"
git push origin v${RELEASE_VERSION}

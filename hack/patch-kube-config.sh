#!/bin/bash

ROOT=$(cd $(dirname $0)/../; pwd)

set -o errexit
set -o nounset
set -o pipefail

USER_TOKEN=$(python ${ROOT}/hack/current-token.py)

cat ${ROOT}/artifacts/kubeconfig-template.yaml | sed -e "s|\${USER_TOKEN}|${USER_TOKEN}|g" > ${ROOT}/artifacts/kubeconfig.yaml

#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

: "${CH_TOKEN:?Environment variable CH_TOKEN must be set}"
: "${GIT_REPOSITORY_URL:?Environment variable GIT_REPO_URL must be set}"
: "${GIT_USERNAME:?Environment variable GIT_USERNAME must be set}"
: "${GIT_EMAIL:?Environment variable GIT_EMAIL must be set}"
: "${GIT_REPOSITORY_NAME:?Environment variable GIT_REPOSITORY_NAME must be set}"

readonly REPO_ROOT="${REPO_ROOT:-$(git rev-parse --show-toplevel)}"
IMG="${IMG:-flink-operator:latest}"

main() {
    pushd "$REPO_ROOT" > /dev/null

    echo "Fetching tags..."
    git fetch --tags

    local latest_tag
    latest_tag=$(find_latest_tag)

    local latest_tag_rev
    latest_tag_rev=$(git rev-parse --verify "$latest_tag")
    echo "$latest_tag_rev $latest_tag (latest tag)"

    local head_rev
    head_rev=$(git rev-parse --verify HEAD)
    echo "$head_rev HEAD"

    if [[ "$latest_tag_rev" == "$head_rev" ]]; then
        echo "No code changes. Nothing to release."
        exit
    fi

    rm -rf .deploy
    mkdir -p .deploy

    echo "Identifying changed charts since tag '$latest_tag'..."

    local changed_charts=()
    readarray -t changed_charts <<< "$(git diff --find-renames --name-only "$latest_tag_rev" -- helm-chart | cut -d '/' -f 2 | uniq)"

    if [[ -n "${changed_charts[*]}" ]]; then
        git clone https://github.com/GoogleCloudPlatform/flink-on-k8s-operator.git
        sed -e 's#image: .*#image: '"${IMG}"'#' flink-on-k8s-operator/config/default/manager_image_patch.template >flink-on-k8s-operator/config/default/manager_image_patch.yaml
        sed -i '/- \.\.\/crd/d' flink-on-k8s-operator/config/default/kustomization.yaml
        kustomize build flink-on-k8s-operator/config/default | tee flink-operator.yaml
        sed -i '1s/^/{{- if .Values.rbac.create }}\n/' flink-operator.yaml
        sed -i -e "\$a{{- end }}\n" flink-operator.yaml
        sed -i 's/flink-operator-system/{{ .Values.flinkOperatorNamespace }}/g' flink-operator.yaml
        sed -i 's/replicas: 1/replicas: {{ .Values.replicas }}/g' flink-operator.yaml
        sed -i "s/$IMG/{{ .Values.operatorImage.name }}/g" flink-operator.yaml
        mv flink-operator.yaml helm-chart/flink-operator/templates/flink-operator.yaml
        cp flink-on-k8s-operator/config/crd/bases/flinkoperator.k8s.io_flinkclusters.yaml helm-chart/flink-operator/templates/flink-cluster-crd.yaml
        sed -i '1s/^/{{ if .Values.rbac.create }}\n/' helm-chart/flink-operator/templates/flink-cluster-crd.yaml
        sed -i -e "\$a{{ end }}\n" helm-chart/flink-operator/templates/flink-cluster-crd.yaml
        sed -i 's/{{$clusterName}}.example.com/clusterName.example.com/g' helm-chart/flink-operator/templates/flink-cluster-crd.yaml

        for chart in "${changed_charts[@]}"; do
            echo "Packaging chart '$chart'..."
            package_chart "helm-chart/$chart"
        done

        release_charts
        sleep 5
        update_index
    else
        echo "Nothing to do. No chart changes detected."
    fi

    popd > /dev/null
}

find_latest_tag() {
    if ! git describe --tags --abbrev=0 2> /dev/null; then
        git rev-list --max-parents=0 --first-parent HEAD
    fi
}

package_chart() {
    local chart="$1"
    helm dependency build "$chart"
    helm package "$chart" --destination .deploy
}

release_charts() {
    chart-releaser upload -o "$GIT_USERNAME" -r "$GIT_REPOSITORY_NAME" -p .deploy
}

update_index() {
    chart-releaser index -o "$GIT_USERNAME" -r "$GIT_REPOSITORY_NAME" -p .deploy/index.yaml

    git config user.email "$GIT_EMAIL"
    git config user.name "$GIT_USERNAME"

    rm -rf flink-on-k8s-operator
    git checkout remote-helm-chart 
    if [ -z "$(git status --porcelain)" ]; then
      echo "nothing to commit."
    else
      git add .
      git commit -m "Update CRDs"
      git push "$GIT_REPOSITORY_URL" remote-helm-chart
    fi

    for file in helm-chart/*/*.md; do
        if [[ -e $file ]]; then
            mkdir -p ".deploy/docs/$(dirname "$file")"
            cp --force "$file" ".deploy/docs/$(dirname "$file")"
        fi
    done

    git checkout gh-pages
    cp --force .deploy/index.yaml index.yaml

    if [[ -e ".deploy/docs/" ]]; then
        mkdir -p charts
        cp --force --recursive .deploy/docs/* charts/
    fi

    git checkout master -- README.md

    if ! git diff --quiet; then
        git add .
        git commit --message="Update index.yaml" --signoff
        git push "$GIT_REPOSITORY_URL" gh-pages
    fi
}

main

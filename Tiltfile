docker_build('mattermost/mattermod', '.', dockerfile='Dockerfile')
docker_build('mattermost/mattermod-jobserver', '.', dockerfile='Dockerfile.jobserver')

k8s_yaml(kustomize('./deploy/overlays/dev'))

k8s_resource('mattermod', port_forwards=[8080, 9000])

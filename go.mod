module github.com/mattermost/mattermost-mattermod

go 1.12

require (
	github.com/aws/aws-sdk-go v1.23.2
	github.com/braintree/manners v0.0.0-20160418043613-82a8879fc5fd
	github.com/cpanato/go-circleci v0.3.1-0.20191014144427-0c2173fc5c76
	github.com/cpanato/golang-jenkins v0.0.0-20181010175751-6a66fc16d07d
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/go-gorp/gorp v2.0.0+incompatible
	github.com/go-sql-driver/mysql v1.4.1
	github.com/gogo/protobuf v1.3.0 // indirect
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/google/go-github/v28 v28.1.1
	github.com/gorilla/mux v1.7.3
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/heroku/docker-registry-client v0.0.0-20190909225348-afc9e1acc3d5
	github.com/mattermost/mattermost-cloud v0.4.0
	github.com/mattermost/mattermost-operator v0.5.2 // indirect
	github.com/mattermost/mattermost-server v0.0.0-20190815184412-135acbb0b001
	github.com/onsi/ginkgo v1.10.1 // indirect
	github.com/onsi/gomega v1.7.0 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/pkg/errors v0.8.1
	github.com/robfig/cron/v3 v3.0.0
	github.com/stretchr/testify v1.4.0
	golang.org/x/crypto v0.0.0-20190911031432-227b76d455e7 // indirect
	golang.org/x/net v0.0.0-20190909003024-a7b16738d86b // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sys v0.0.0-20190911201528-7ad0cfa0b7b5 // indirect
	google.golang.org/appengine v1.6.2 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	k8s.io/api v0.0.0-20190814101207-0772a1bdf941 // indirect
)

replace git.apache.org/thrift.git => github.com/apache/thrift v0.12.0

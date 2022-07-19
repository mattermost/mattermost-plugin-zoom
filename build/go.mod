module github.com/mattermost/mattermost-plugin-starter-template/build

go 1.12

require (
	github.com/mattermost/mattermost-server/v6 v6.5.0
	github.com/mholt/archiver/v3 v3.5.1
	github.com/pkg/errors v0.9.1
)

replace github.com/mattermost/mattermost-server/v6 => ../../mattermost-server

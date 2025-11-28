module github.com/mpc_hsm/orchestrator

go 1.24.0

require (
	github.com/99designs/gqlgen v0.17.83
	github.com/gorilla/websocket v1.5.3
	github.com/mpc_hsm/node v0.0.0
	github.com/vektah/gqlparser/v2 v2.5.31
	google.golang.org/grpc v1.68.1
)

replace github.com/mpc_hsm/node => ../node

require (
	github.com/agnivade/levenshtein v1.2.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/sosodev/duration v1.3.1 // indirect
	github.com/urfave/cli/v3 v3.6.0 // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/net v0.46.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/tools v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240903143218-8af14fe29dc1 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

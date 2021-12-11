module flight

go 1.15

require (
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/gobwas/httphead v0.0.0-20180130184737-2c6c146eadee // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/gobwas/ws v1.0.3 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/juju/fslock v0.0.0-20160525022230-4d5c94c67b4b // indirect
	github.com/micro/micro/v3 v3.8.0
	github.com/minio/minio-go/v7 v7.0.5 // indirect
	github.com/rhysd/go-github-selfupdate v1.2.2 // indirect
	github.com/soheilhy/cmux v0.1.4 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
)

// This can be removed once etcd becomes go gettable, version 3.4 and 3.5 is not,
// see https://github.com/etcd-io/etcd/issues/11154 and https://github.com/etcd-io/etcd/issues/11931.
replace google.golang.org/grpc => google.golang.org/grpc v1.26.0

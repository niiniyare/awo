syntax="proto3";

package example;

import "google/api/annotations.proto";
import "protoc-gen-openapiv2/options/annotations.proto";

// Defines the import path that should be used to import the generated package,
// and the package name.
option go_package = "github.com/Abdirahman0022/cawo/proto;example";

// These annotations are used when generating the OpenAPI file.
option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_swagger) = {
  info: {
    version: "1.0";
  };
  external_docs: {
    url: "https://github.com/Abdirahman0022/cawo";
    description: "gRPC-gateway boilerplate repository";
  }
  schemes: HTTPS;
};

service UserService {
  rpc AddUser(AddUserRequest) returns (User) {
    option (google.api.http) = {
      // Route to this method from POST requests to /api/v1/users
      post: "/api/v1/users"
      body: "*"
    };
    option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_operation) = {
      summary: "Add a user"
      description: "Add a user to the server."
      tags: "Users"
    };
  }
  rpc ListUsers(ListUsersRequest) returns (stream User) {
    option (google.api.http) = {
      // Route to this method from GET requests to /api/v1/users
      get: "/api/v1/users"
    };
    option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_operation) = {
      summary: "List users"
      description: "List all users on the server."
      tags: "Users"
    };
  }
}

message AddUserRequest {}

message ListUsersRequest {}

message User {
  string id = 1;
}

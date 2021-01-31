## proto

Protocol buffer definitions for examples.

### Compilation

Execute the following command under the root directory of the project.

```
$ protoc --go_out=plugins=grpc,paths=source_relative:. ./examples/proto/*.proto
```
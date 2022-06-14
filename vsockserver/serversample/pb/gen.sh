protoc --gogofaster_out=paths=source_relative:. hello.proto
ls *.pb.go | xargs -n1 -IX bash -c 'sed s/,omitempty// X > X.tmp && mv X{.tmp,}'

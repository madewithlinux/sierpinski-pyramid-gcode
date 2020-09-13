clean:
    rm -f sierpinski-pyramid-gcode
    rm -f sierpinski-pyramid-gcode.darwin
    rm -f sierpinski-pyramid-gcode.linux
    rm -f sierpinski-pyramid-gcode.windows.exe

ci:
    #!/bin/bash
    diff -u <(echo -n) <(gofmt -d .)
    go vet ./...
    go test -v -race ./...
    gox -os="linux darwin windows" -arch="amd64" -output='sierpinski-pyramid-gcode.{{"{{"}}.OS}}' -ldflags "-X main.Rev=`git rev-parse HEAD` -X main.Version=`git describe --tags`" -verbose ./...

fmt:
    go fmt -x ./...

test:
    go test -v ./...


gif:
    go run ./cmd/layer_animation/
    ffmpeg -f image2 -i renders/layer_animation_%05d.png layer_animation.gif
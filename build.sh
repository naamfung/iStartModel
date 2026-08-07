rm -rf *.exe*
# CC='zig cc' go build -ldflags '-H windowsgui -extldflags "-Wl,--subsystem,windows"'
go build -ldflags '-H windowsgui -extldflags "-Wl,--subsystem,windows"'

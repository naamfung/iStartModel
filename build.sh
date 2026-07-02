rm -rf *.exe*
rm -rf *.exe
# CC='zig cc' go build -ldflags '-H windowsgui -extldflags "-Wl,--subsystem,windows"' -o iStartWork.exe
go build -ldflags '-H windowsgui -extldflags "-Wl,--subsystem,windows"' -o iStartWork.exe

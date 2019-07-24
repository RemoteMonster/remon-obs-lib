# libremonobs

This is a library for [ROS RemoteMonster OBS Studio](https://github.com/RemoteMonster/remon-obs-studio) to use for Signalling and WebRTC broadcast.

Thanks to [Pion WebRTC](https://github.com/pion/webrtc) project team.

## Status

This project is currently experimental.

The contents of this project are subject to change according to Remon OBS Studio (ROS) status.

## Install

This package supports Go module.
```
git clone https://github.com/Remotemonster/remon-obs-lib.git
```
```
cd remon-obs-lib
```

on linux
```
go build -buildmode=c-shared -o libremonobs.so .
```
on windows
```
go build -buildmode=c-shared -o libremonobs.dll .
```

Copy the generated libremonobs.h and libremonobs.so (or dll) files to the deps / libremonobs directory in remon-obs-studio project.

However, Visual Studio can not compile the auto-generated libremonobs.h file, so it may need to be modified.


## License

GPL




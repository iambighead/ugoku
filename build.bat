@echo off
IF [%1]==[] GOTO usage
SET VERSION=%1
echo cleanup
del /s /q build
rmdir build
mkdir build
echo building
go build
move ugoku.exe build\
echo copy config files
copy config.template.win.ini build\config.template.ini
copy config.template.win.ini build\config.ini
echo zipping up
cd build
7z a ugoku-win-%1.zip *
dir
goto end
:usage
echo usage: build.bat [version]
:end
if [ -z ${1+x} ];
then
    echo "Usage: build.sh [version]"
else
    version=$1
    echo cleanup
    rm -rf build
    mkdir build
    echo building
    CGO_ENABLED=0 go build
    mv ugoku build/
    echo copy config files
    cp config.template.linux.ini build/config.template.ini
    cp config.template.linux.ini build/config.ini
    cp ugoku.service build/
    echo zipping up
    cd build
    zip ugoku-linux-$version.zip *
    ls -l
fi

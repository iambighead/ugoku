# Ugoku

## Overview

A SFTP client written in Golang. The objective is to allow automated machine to machine file transfer via SFTP.

## Features

- SFTP Downloader (SFTP server to local folder, files are removed after download)
- SFTP Uploader (local folder to SFTP server, files are removed after upload)
- Sync to Local (mirror files from SFTP server, files are not removed)
- Sync to Server (mirror files to SFTP server, files are not removed)
- Streamer (SFTP Server to Server transfer via Ugoku as bridge)
- build in logger

## Todo
- Archiving support for downloader/uploader/streamer
- Key login

## Why the name Ugoku

Ugoku in Japanese means move. This app moves files around, hence the name. Bonus point it has "go" in the name.

## License

Ugoku is available under the
[MIT license](https://opensource.org/licenses/MIT).See
[LICENSE](https://github.com/iambighead/ugoku/blob/HEAD/LICENSE) for the full
license text.
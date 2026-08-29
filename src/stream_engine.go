package src

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type ThirdPartyStreamEngine struct{}

const (
	maxThirdPartyProcessRestarts = 3
	thirdPartyProcessRestartWait = 500 * time.Millisecond
)

func (ThirdPartyStreamEngine) Run(streamID int, playlistID string, useBackup bool, backupNumber int) {
	if p, ok := BufferInformation.Load(playlistID); ok {

		var playlist = p.(Playlist)
		var debug string
		var bufferSize = Settings.BufferSize * 1024
		var stream = playlist.Streams[streamID]
		var streamStatus = make(chan bool)

		var tmpFolder = playlist.Streams[streamID].Folder
		var segmentWriter = NewThirdPartySegmentWriter(tmpFolder, bufferSize)
		var url, selectedBackup = selectThirdPartyStreamURL(stream, useBackup, backupNumber)
		if selectedBackup > 0 {
			showHighlight(fmt.Sprintf("START OF BACKUP %d STREAM", selectedBackup))
			showInfo(fmt.Sprintf("Backup Channel %d URL: %s", selectedBackup, redactStreamURL(url)))
		}

		stream.Status = false

		engine, forcedHTTP, supportedEngine := resolveThirdPartyEngine(playlist, url)
		if !supportedEngine {
			return
		}
		if forcedHTTP {
			showInfo("Forcing URL to HTTP for FFMPEG: " + redactStreamURL(engine.URL))
		}

		if err := segmentWriter.Reset(); err != nil {
			ShowError(err, 0)
			killClientConnection(streamID, playlistID, false)
			addThirdPartyErrorToStream(streamID, playlistID, stream, backupNumber, err)
			return
		}

		err := checkFile(engine.Path)
		if err != nil {
			ShowError(err, 0)
			killClientConnection(streamID, playlistID, false)
			addThirdPartyErrorToStream(streamID, playlistID, stream, backupNumber, err)
			return
		}

		showInfo(fmt.Sprintf("%s path:%s", engine.BufferType, engine.Path))
		showInfo("Streaming URL:" + redactStreamURL(engine.URL))

		if err := segmentWriter.CreateCurrent(); err != nil {
			ShowError(err, 0)
			killClientConnection(streamID, playlistID, false)
			addThirdPartyErrorToStream(streamID, playlistID, stream, backupNumber, err)
			return
		}

		args, err := buildThirdPartyArgs(engine.BufferType, engine.Options, engine.URL, playlist)
		if err != nil {
			ShowError(err, 0)
			killClientConnection(streamID, playlistID, false)
			addThirdPartyErrorToStream(streamID, playlistID, stream, backupNumber, err)
			return
		}

		debug = fmt.Sprintf("BUFFER DEBUG: %s:%s %s", engine.BufferType, engine.Path, RedactCommandArgs(args))
		showDebug(debug, 1)

		if !stream.Status {
			showInfo(engine.BufferType + ":Processing data")
		}

		startProcess := func() (*ThirdPartyProcess, *bufio.Reader, error) {
			process, err := StartThirdPartyProcess(engine.Path, args)
			if err != nil {
				return nil, nil, err
			}

			go func() {

				// Display log data from the process in debug mode 1.
				scanner := bufio.NewScanner(process.Stderr())
				scanner.Split(bufio.ScanLines)

				for scanner.Scan() {

					debug = fmt.Sprintf("%s log:%s", engine.BufferType, strings.TrimSpace(scanner.Text()))

					select {
					case <-streamStatus:
						showDebug(debug, 1)
					default:
						showInfo(debug)
					}

					time.Sleep(time.Duration(10) * time.Millisecond)

				}

			}()

			return process, bufio.NewReader(process.Stdout()), nil
		}

		process, reader, err := startProcess()
		if err != nil {
			ShowError(err, 0)
			killClientConnection(streamID, playlistID, false)
			addThirdPartyErrorToStream(streamID, playlistID, stream, backupNumber, err)
			return
		}

		if err := segmentWriter.OpenCurrent(); err != nil {
			ShowError(err, 0)
			killClientConnection(streamID, playlistID, false)
			addThirdPartyErrorToStream(streamID, playlistID, stream, backupNumber, err)
			process.Terminate()
			return
		}
		defer segmentWriter.Close()

		buffer := make([]byte, 1024*4)

		startedAt := time.Now()
		startupTimeout := thirdPartyStartupTimeout()
		restarts := 0

		for {

			if time.Since(startedAt) >= startupTimeout && segmentWriter.Segment() == 1 {
				err = errors.New("Timeout")
				ShowError(err, 4006)
				killClientConnection(streamID, playlistID, false)
				addThirdPartyErrorToStream(streamID, playlistID, stream, backupNumber, err)
				process.Terminate()
				segmentWriter.Close()
				return
			}

			if segmentWriter.CurrentSize() == 0 && !stream.Status {
				showInfo("Streaming Status:Receive data from " + engine.BufferType)
			}

			if !clientConnection(stream) {
				segmentWriter.Close()
				process.Terminate()
				return
			}

			n, err := reader.Read(buffer)
			if err == io.EOF {
				err = withThirdPartyStderr(errors.New(engine.BufferType+" error"), process)
				process.Terminate()

				if shouldRestartThirdPartyProcess(stream, restarts, backupNumber) && clientConnection(stream) {
					restarts++
					ShowError(err, 1204)
					showInfo(fmt.Sprintf("Streaming Status:%s ended after startup; restarting source process (%d/%d)", engine.BufferType, restarts, maxThirdPartyProcessRestarts))
					time.Sleep(thirdPartyProcessRestartWait)

					process, reader, err = startProcess()
					if err != nil {
						ShowError(err, 0)
						killClientConnection(streamID, playlistID, false)
						addThirdPartyErrorToStream(streamID, playlistID, stream, backupNumber, err)
						return
					}
					continue
				}

				addThirdPartyErrorToStream(streamID, playlistID, stream, backupNumber, err)
				ShowError(err, 1204)
				time.Sleep(time.Duration(500) * time.Millisecond)
				clientConnection(stream)
				return
			}
			if err != nil {
				err = withThirdPartyStderr(err, process)
				ShowError(err, 0)
				killClientConnection(streamID, playlistID, false)
				addThirdPartyErrorToStream(streamID, playlistID, stream, backupNumber, err)
				process.Terminate()
				return
			}

			if _, err := segmentWriter.Write(buffer[:n]); err != nil {
				ShowError(err, 0)
				killClientConnection(streamID, playlistID, false)
				addThirdPartyErrorToStream(streamID, playlistID, stream, backupNumber, err)
				process.Terminate()
				return
			}

			if segmentWriter.ShouldRotate() {

				if segmentWriter.Segment() == 1 && !stream.Status {
					close(streamStatus)
					showInfo(fmt.Sprintf("Streaming Status:Buffering data from %s", engine.BufferType))
				}

				if !stream.Status {
					Lock.Lock()
					stream.Status = true
					playlist.Streams[streamID] = stream
					BufferInformation.Store(playlistID, playlist)
					Lock.Unlock()
				}

				if err := segmentWriter.Rotate(); err != nil {
					ShowError(err, 0)
					killClientConnection(streamID, playlistID, false)
					addThirdPartyErrorToStream(streamID, playlistID, stream, backupNumber, err)
					process.Terminate()
					return
				}

			}

		}

	}

}

func thirdPartyBuffer(streamID int, playlistID string, useBackup bool, backupNumber int) {
	ThirdPartyStreamEngine{}.Run(streamID, playlistID, useBackup, backupNumber)
}

func withThirdPartyStderr(err error, process *ThirdPartyProcess) error {
	if err == nil || process == nil {
		return err
	}

	stderr := process.StderrTail()
	if stderr == "" {
		return err
	}

	return fmt.Errorf("%w: %s", err, stderr)
}

func shouldRestartThirdPartyProcess(stream ThisStream, restarts int, backupNumber int) bool {
	if !stream.Status {
		return false
	}
	if restarts >= maxThirdPartyProcessRestarts {
		return false
	}
	if _, ok := nextBackupNumber(stream, backupNumber); ok {
		return false
	}
	return true
}

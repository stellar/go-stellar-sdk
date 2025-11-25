package ledgerbackend

import (
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"time"

	"github.com/stellar/go-stellar-sdk/support/log"
)

type isDir interface {
	IsDir() bool
}

type systemCaller interface {
	removeAll(path string) error
	writeFile(filename string, data []byte, perm fs.FileMode) error
	mkdirAll(path string, perm os.FileMode) error
	stat(name string) (isDir, error)
	command(ctx context.Context, name string, arg ...string) cmdI
}

type realSystemCaller struct{}

func (realSystemCaller) removeAll(path string) error {
	return os.RemoveAll(path)
}

func (realSystemCaller) writeFile(filename string, data []byte, perm fs.FileMode) error {
	return ioutil.WriteFile(filename, data, perm)
}

func (realSystemCaller) mkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (realSystemCaller) stat(name string) (isDir, error) {
	return os.Stat(name)
}

func (realSystemCaller) command(ctx context.Context, name string, arg ...string) cmdI {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.Cancel = func() error {
		return cmd.Process.Signal(os.Interrupt)
	}
	cmd.WaitDelay = time.Second * 10
	return &realCmd{Cmd: cmd}
}

type cmdI interface {
	Output() ([]byte, error)
	Wait() error
	Start() error
	Run() error
	setDir(dir string)
	setLogLineWriter(logWriter *logLineWriter)
	setExtraFiles([]*os.File)
}

type realCmd struct {
	*exec.Cmd
	logWriter *logLineWriter
}

func (r *realCmd) setDir(dir string) {
	r.Cmd.Dir = dir
}

func (r *realCmd) setLogLineWriter(logWriter *logLineWriter) {
	r.logWriter = logWriter
}

func (r *realCmd) setExtraFiles(extraFiles []*os.File) {
	r.ExtraFiles = extraFiles
}

func (r *realCmd) Start() error {
	if r.logWriter != nil {
		r.Cmd.Stdout = r.logWriter
		r.Cmd.Stderr = r.logWriter
		r.logWriter.Start()
	}
	err := r.Cmd.Start()
	if err != nil && r.logWriter != nil {
		r.logWriter.Close()
	}
	return err
}

func (r *realCmd) Run() error {
	if r.logWriter != nil {
		r.Cmd.Stdout = r.logWriter
		r.Cmd.Stderr = r.logWriter
		r.logWriter.Start()
	}
	err := r.Cmd.Run()
	if r.logWriter != nil {
		r.logWriter.Close()
	}
	return err
}

func (r *realCmd) Wait() error {
	err := r.Cmd.Wait()
	if r.logWriter != nil {
		r.logWriter.Close()
	}
	return err
}

type coreCmdFactory struct {
	log            *log.Entry
	systemCaller   systemCaller
	executablePath string
	dir            workingDir
	nonce          string
}

func newCoreCmdFactory(r *stellarCoreRunner, dir workingDir) coreCmdFactory {
	return coreCmdFactory{
		log:            r.log,
		systemCaller:   r.systemCaller,
		executablePath: r.executablePath,
		dir:            dir,
		nonce: fmt.Sprintf(
			"captive-stellar-core-%x",
			rand.New(rand.NewSource(time.Now().UnixNano())).Uint64(),
		),
	}
}

func (c coreCmdFactory) newCmd(ctx context.Context, mode stellarCoreRunnerMode, redirectOutputToLogs bool, params ...string) (cmdI, error) {
	if err := c.dir.createIfNotExists(); err != nil {
		return nil, err
	}

	if err := c.dir.writeConf(mode); err != nil {
		return nil, fmt.Errorf("error writing configuration: %w", err)
	}

	allParams := []string{"--conf", c.dir.getConfFileName()}
	if redirectOutputToLogs {
		allParams = append(allParams, "--console")
	}
	allParams = append(allParams, params...)
	cmd := c.systemCaller.command(ctx, c.executablePath, allParams...)
	cmd.setDir(c.dir.path)
	if redirectOutputToLogs {
		cmd.setLogLineWriter(newLogLineWriter(c.log))
	}
	return cmd, nil
}

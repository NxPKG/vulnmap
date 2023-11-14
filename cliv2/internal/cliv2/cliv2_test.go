package cliv2_test

import (
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"sort"
	"testing"
	"time"

	"github.com/khulnasoft-lab/go-application-framework/pkg/configuration"

	"github.com/khulnasoft-lab/vulnmap/cliv2/internal/cliv2"
	"github.com/khulnasoft-lab/vulnmap/cliv2/internal/constants"
	"github.com/khulnasoft-lab/vulnmap/cliv2/internal/proxy"
	"github.com/khulnasoft-lab/vulnmap/cliv2/internal/utils"

	"github.com/stretchr/testify/assert"
)

var discardLogger = log.New(io.Discard, "", 0)

func getCacheDir(t *testing.T) string {
	t.Helper()
	cacheDir := path.Join(t.TempDir(), "vulnmap")
	err := os.MkdirAll(cacheDir, 0755)
	assert.Nil(t, err)
	return cacheDir
}

func Test_PrepareV1EnvironmentVariables_Fill_and_Filter(t *testing.T) {

	input := []string{
		"something=1",
		"in=2",
		"here=3=2",
		"NO_PROXY=noProxy",
		"HTTPS_PROXY=httpsProxy",
		"HTTP_PROXY=httpProxy",
		"NPM_CONFIG_PROXY=something",
		"NPM_CONFIG_HTTPS_PROXY=something",
		"NPM_CONFIG_HTTP_PROXY=something",
		"npm_config_no_proxy=something",
		"ALL_PROXY=something",
	}
	expected := []string{"something=1",
		"in=2",
		"here=3=2",
		"VULNMAP_INTEGRATION_NAME=foo",
		"VULNMAP_INTEGRATION_VERSION=bar",
		"HTTP_PROXY=proxy",
		"HTTPS_PROXY=proxy",
		"NODE_EXTRA_CA_CERTS=cacertlocation",
		"VULNMAP_SYSTEM_NO_PROXY=noProxy",
		"VULNMAP_SYSTEM_HTTP_PROXY=httpProxy",
		"VULNMAP_SYSTEM_HTTPS_PROXY=httpsProxy",
		"VULNMAP_INTERNAL_ORGID=orgid",
		"NO_PROXY=" + constants.VULNMAP_INTERNAL_NO_PROXY + ",noProxy",
	}

	actual, err := cliv2.PrepareV1EnvironmentVariables(input, "foo", "bar", "proxy", "cacertlocation", "orgid")

	sort.Strings(expected)
	sort.Strings(actual)
	assert.Equal(t, expected, actual)
	assert.Nil(t, err)
}

func Test_PrepareV1EnvironmentVariables_DontOverrideExistingIntegration(t *testing.T) {

	input := []string{"something=1", "in=2", "here=3", "VULNMAP_INTEGRATION_NAME=exists", "VULNMAP_INTEGRATION_VERSION=already"}
	expected := []string{
		"something=1",
		"in=2",
		"here=3",
		"VULNMAP_INTEGRATION_NAME=exists",
		"VULNMAP_INTEGRATION_VERSION=already",
		"HTTP_PROXY=proxy",
		"HTTPS_PROXY=proxy",
		"NODE_EXTRA_CA_CERTS=cacertlocation",
		"VULNMAP_SYSTEM_NO_PROXY=",
		"VULNMAP_SYSTEM_HTTP_PROXY=",
		"VULNMAP_SYSTEM_HTTPS_PROXY=",
		"VULNMAP_INTERNAL_ORGID=orgid",
		"NO_PROXY=" + constants.VULNMAP_INTERNAL_NO_PROXY,
	}

	actual, err := cliv2.PrepareV1EnvironmentVariables(input, "foo", "bar", "proxy", "cacertlocation", "orgid")

	sort.Strings(expected)
	sort.Strings(actual)
	assert.Equal(t, expected, actual)
	assert.Nil(t, err)
}

func Test_PrepareV1EnvironmentVariables_OverrideProxyAndCerts(t *testing.T) {

	input := []string{"something=1", "in=2", "here=3", "http_proxy=exists", "https_proxy=already", "NODE_EXTRA_CA_CERTS=again", "no_proxy=312123"}
	expected := []string{
		"something=1",
		"in=2",
		"here=3",
		"VULNMAP_INTEGRATION_NAME=foo",
		"VULNMAP_INTEGRATION_VERSION=bar",
		"HTTP_PROXY=proxy",
		"HTTPS_PROXY=proxy",
		"NODE_EXTRA_CA_CERTS=cacertlocation",
		"VULNMAP_SYSTEM_NO_PROXY=312123",
		"VULNMAP_SYSTEM_HTTP_PROXY=exists",
		"VULNMAP_SYSTEM_HTTPS_PROXY=already",
		"VULNMAP_INTERNAL_ORGID=orgid",
		"NO_PROXY=" + constants.VULNMAP_INTERNAL_NO_PROXY + ",312123",
	}

	actual, err := cliv2.PrepareV1EnvironmentVariables(input, "foo", "bar", "proxy", "cacertlocation", "orgid")

	sort.Strings(expected)
	sort.Strings(actual)
	assert.Equal(t, expected, actual)
	assert.Nil(t, err)
}

func Test_PrepareV1EnvironmentVariables_Fail_DontOverrideExisting(t *testing.T) {

	input := []string{"something=1", "in=2", "here=3", "VULNMAP_INTEGRATION_NAME=exists"}
	expected := input

	actual, err := cliv2.PrepareV1EnvironmentVariables(input, "foo", "bar", "unused", "unused", "orgid")

	sort.Strings(expected)
	sort.Strings(actual)
	assert.Equal(t, expected, actual)

	warn, ok := err.(cliv2.EnvironmentWarning)
	assert.True(t, ok)
	assert.NotNil(t, warn)
}

func getProxyInfoForTest() *proxy.ProxyInfo {
	return &proxy.ProxyInfo{
		Port:                1000,
		Password:            "foo",
		CertificateLocation: "certLocation",
	}
}

func Test_prepareV1Command(t *testing.T) {
	expectedArgs := []string{"hello", "world"}
	cacheDir := getCacheDir(t)
	config := configuration.NewInMemory()
	config.Set(configuration.CACHE_PATH, cacheDir)
	cli, _ := cliv2.NewCLIv2(config, discardLogger)

	vulnmapCmd, err := cli.PrepareV1Command(
		"someExecutable",
		expectedArgs,
		getProxyInfoForTest(),
		"name",
		"version",
	)

	assert.Contains(t, vulnmapCmd.Env, "VULNMAP_INTEGRATION_NAME=name")
	assert.Contains(t, vulnmapCmd.Env, "VULNMAP_INTEGRATION_VERSION=version")
	assert.Contains(t, vulnmapCmd.Env, "HTTPS_PROXY=http://vulnmapcli:foo@127.0.0.1:1000")
	assert.Contains(t, vulnmapCmd.Env, "NODE_EXTRA_CA_CERTS=certLocation")
	assert.Equal(t, expectedArgs, vulnmapCmd.Args[1:])
	assert.Nil(t, err)
}

func Test_extractOnlyOnce(t *testing.T) {
	cacheDir := getCacheDir(t)
	tmpDir := utils.GetTemporaryDirectory(cacheDir, cliv2.GetFullVersion())
	config := configuration.NewInMemory()
	config.Set(configuration.CACHE_PATH, cacheDir)

	assert.NoDirExists(t, tmpDir)

	// create instance under test
	cli, _ := cliv2.NewCLIv2(config, discardLogger)

	// run once
	assert.Nil(t, cli.Init())
	cli.Execute(getProxyInfoForTest(), []string{"--help"})
	assert.FileExists(t, cli.GetBinaryLocation())
	fileInfo1, _ := os.Stat(cli.GetBinaryLocation())

	// sleep shortly to ensure that ModTimes would be different
	time.Sleep(500 * time.Millisecond)

	// run twice
	assert.Nil(t, cli.Init())
	cli.Execute(getProxyInfoForTest(), []string{"--help"})
	assert.FileExists(t, cli.GetBinaryLocation())
	fileInfo2, _ := os.Stat(cli.GetBinaryLocation())

	assert.Equal(t, fileInfo1.ModTime(), fileInfo2.ModTime())
}

func Test_init_extractDueToInvalidBinary(t *testing.T) {
	cacheDir := getCacheDir(t)
	tmpDir := utils.GetTemporaryDirectory(cacheDir, cliv2.GetFullVersion())
	config := configuration.NewInMemory()
	config.Set(configuration.CACHE_PATH, cacheDir)

	assert.NoDirExists(t, tmpDir)

	// create instance under test
	cli, _ := cliv2.NewCLIv2(config, discardLogger)

	// fill binary with invalid data
	_ = os.MkdirAll(tmpDir, 0755)
	_ = os.WriteFile(cli.GetBinaryLocation(), []byte("Writing some strings"), 0755)
	fileInfo1, _ := os.Stat(cli.GetBinaryLocation())

	// prove that we can't execute the invalid binary
	_, binError := exec.Command(cli.GetBinaryLocation(), "--help").Output()
	assert.NotNil(t, binError)

	// sleep shortly to ensure that ModTimes would be different
	time.Sleep(500 * time.Millisecond)

	// run init to ensure that the file system is being setup correctly
	initError := cli.Init()
	assert.Nil(t, initError)

	// execute to test that the cli can run successfully
	assert.FileExists(t, cli.GetBinaryLocation())

	fileInfo2, _ := os.Stat(cli.GetBinaryLocation())

	assert.NotEqual(t, fileInfo1.ModTime(), fileInfo2.ModTime())
}

func Test_executeRunV2only(t *testing.T) {
	expectedReturnCode := 0

	cacheDir := getCacheDir(t)
	tmpDir := utils.GetTemporaryDirectory(cacheDir, cliv2.GetFullVersion())
	config := configuration.NewInMemory()
	config.Set(configuration.CACHE_PATH, cacheDir)

	assert.NoDirExists(t, tmpDir)

	// create instance under test
	cli, _ := cliv2.NewCLIv2(config, discardLogger)
	assert.Nil(t, cli.Init())

	actualReturnCode := cliv2.DeriveExitCode(cli.Execute(getProxyInfoForTest(), []string{"--version"}))
	assert.Equal(t, expectedReturnCode, actualReturnCode)
	assert.FileExists(t, cli.GetBinaryLocation())

}

func Test_executeUnknownCommand(t *testing.T) {
	expectedReturnCode := constants.VULNMAP_EXIT_CODE_ERROR

	cacheDir := getCacheDir(t)
	config := configuration.NewInMemory()
	config.Set(configuration.CACHE_PATH, cacheDir)

	// create instance under test
	cli, _ := cliv2.NewCLIv2(config, discardLogger)
	assert.Nil(t, cli.Init())

	actualReturnCode := cliv2.DeriveExitCode(cli.Execute(getProxyInfoForTest(), []string{"bogusCommand"}))
	assert.Equal(t, expectedReturnCode, actualReturnCode)
}

func Test_clearCache(t *testing.T) {
	cacheDir := getCacheDir(t)
	config := configuration.NewInMemory()
	config.Set(configuration.CACHE_PATH, cacheDir)

	// create instance under test
	cli, _ := cliv2.NewCLIv2(config, discardLogger)
	assert.Nil(t, cli.Init())

	// create folders and files in cache dir
	versionWithV := path.Join(cli.CacheDirectory, "v1.914.0")
	versionNoV := path.Join(cli.CacheDirectory, "1.1048.0-dev.2401acbc")
	lockfile := path.Join(cli.CacheDirectory, "v1.914.0.lock")
	randomFile := path.Join(versionNoV, "filename")
	currentVersion := cli.GetBinaryLocation()

	_ = os.Mkdir(versionWithV, 0755)
	_ = os.Mkdir(versionNoV, 0755)
	_ = os.WriteFile(randomFile, []byte("Writing some strings"), 0666)
	_ = os.WriteFile(lockfile, []byte("Writing some strings"), 0666)

	// clear cache
	err := cli.ClearCache()
	assert.Nil(t, err)

	// check if directories that need to be deleted don't exist
	assert.NoDirExists(t, versionWithV)
	assert.NoDirExists(t, versionNoV)
	assert.NoFileExists(t, randomFile)
	// check if directories that need to exist still exist
	assert.FileExists(t, currentVersion)
	assert.FileExists(t, lockfile)
}

func Test_clearCacheBigCache(t *testing.T) {
	cacheDir := getCacheDir(t)
	config := configuration.NewInMemory()
	config.Set(configuration.CACHE_PATH, cacheDir)

	// create instance under test
	cli, _ := cliv2.NewCLIv2(config, discardLogger)
	assert.Nil(t, cli.Init())

	// create folders and files in cache dir
	dir1 := path.Join(cli.CacheDirectory, "dir1")
	dir2 := path.Join(cli.CacheDirectory, "dir2")
	dir3 := path.Join(cli.CacheDirectory, "dir3")
	dir4 := path.Join(cli.CacheDirectory, "dir4")
	dir5 := path.Join(cli.CacheDirectory, "dir5")
	dir6 := path.Join(cli.CacheDirectory, "dir6")
	currentVersion := cli.GetBinaryLocation()

	_ = os.Mkdir(dir1, 0755)
	_ = os.Mkdir(dir2, 0755)
	_ = os.Mkdir(dir3, 0755)
	_ = os.Mkdir(dir4, 0755)
	_ = os.Mkdir(dir5, 0755)
	_ = os.Mkdir(dir6, 0755)

	// clear cache
	err := cli.ClearCache()
	assert.Nil(t, err)

	// check if directories that need to be deleted don't exist
	assert.NoDirExists(t, dir1)
	assert.NoDirExists(t, dir2)
	assert.NoDirExists(t, dir3)
	assert.NoDirExists(t, dir4)
	assert.NoDirExists(t, dir5)
	// check if directories that need to exist still exist
	assert.DirExists(t, dir6)
	assert.FileExists(t, currentVersion)
}
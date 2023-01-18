package helpers

import (
	"acceptance/config"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry/cf-test-helpers/v2/cf"
	"github.com/cloudfoundry/cf-test-helpers/v2/generator"
	cfh "github.com/cloudfoundry/cf-test-helpers/v2/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

const AppResidentSize = 55

type AppInfo struct {
	Name  string
	Guid  string
	State string
}

func GetAllStartedApp(cfg *config.Config, org, space, appPrefix string) []AppInfo {
	var startedApps []AppInfo
	rawApps := getRawApps(space, org, cfg.DefaultTimeoutDuration())
	for _, rawApp := range rawApps {
		appName := rawApp.Name
		appState := rawApp.State
		if strings.Contains(appName, appPrefix) && appState == "STARTED" {
			appInfo := &AppInfo{
				Name:  appName,
				Guid:  rawApp.GUID,
				State: appState,
			}
			startedApps = append(startedApps, *appInfo)
		}
	}
	return startedApps
}

func GetApps(cfg *config.Config, orgGuid, spaceGuid string, prefix string) []string {
	rawApps := getRawApps(spaceGuid, orgGuid, cfg.DefaultTimeoutDuration())
	return filterByPrefix(prefix, getNames(rawApps))
}

func getRawAppsByPage(spaceGuid string, orgGuid string, page int, timeout time.Duration) cfResourceObject {
	var appsResponse cfResourceObject
	rawApps := cf.Cf("curl", "/v3/apps?space_guids="+spaceGuid+"&organization_guids="+orgGuid+"&page="+strconv.Itoa(page)).Wait(timeout)
	Expect(rawApps).To(Exit(0), "unable to get apps")
	err := json.Unmarshal(rawApps.Out.Contents(), &appsResponse)
	Expect(err).ShouldNot(HaveOccurred())
	return appsResponse
}

func getRawApps(spaceGuid string, orgGuid string, timeout time.Duration) []cfResource {
	var rawApps []cfResource
	totalPages := 1

	for page := 1; page <= totalPages; page++ {
		var appsResponse = getRawAppsByPage(spaceGuid, orgGuid, page, timeout)
		GinkgoWriter.Println(appsResponse.Pagination.TotalPages)
		totalPages = appsResponse.Pagination.TotalPages
		rawApps = append(rawApps, appsResponse.Resources...)
	}

	return rawApps
}

func SendMetric(cfg *config.Config, appName string, metric int) {
	cfh.CurlApp(cfg, appName, fmt.Sprintf("/custom-metrics/test_metric/%d", metric), "-f")
}

func StartAppWithErr(appName string, timeout time.Duration) error {
	startApp := func() error {
		var err error
		var startApp = cf.Cf("start", appName).Wait(timeout)
		if startApp.ExitCode() != 0 {
			err = fmt.Errorf("failed to start an app: %s  %s", appName, string(startApp.Err.Contents()))
		}
		return err
	}
	return Retry(defaultRetryAttempt, defaultRetryAfter, startApp)
}
func StartApp(appName string, timeout time.Duration) bool {
	startApp := cf.Cf("start", appName).Wait(timeout)
	if startApp.ExitCode() != 0 {
		cf.Cf("logs", appName, "--recent").Wait(2 * time.Minute)
	}
	return Expect(startApp).To(Exit(0))
}

func CreateTestApp(cfg *config.Config, appType string, initialInstanceCount int) string {
	appName := generator.PrefixedRandomName(cfg.Prefix, appType)
	By("Creating test app")
	CreateTestAppByName(*cfg, appName, initialInstanceCount)
	return appName
}
func CreateDroplet(cfg config.Config) string {
	appName := "deleteme"
	tmpDir, err := os.CreateTemp("", "droplet")
	dropletPath := fmt.Sprintf("%s.tgz", tmpDir.Name())
	Expect(err).NotTo(HaveOccurred())
	CreateTestAppByName(cfg, appName, 1)
	StartApp(appName, cfg.CfPushTimeoutDuration())
	downloadDroplet := cf.Cf("download-droplet", appName, "--path", dropletPath).Wait(cfg.DefaultTimeoutDuration())
	DeleteTestApp(appName, cfg.DefaultTimeoutDuration())
	Expect(downloadDroplet).To(Exit(0), "failed download droplet")

	return dropletPath
}

func CreateTestAppFromDropletByName(cfg *config.Config, dropletPath string, appName string, initialInstanceCount int) error {
	return createTestApp(*cfg, appName, initialInstanceCount, "--droplet", dropletPath)
}

func createTestApp(cfg config.Config, appName string, initialInstanceCount int, args ...string) error {
	setNodeTLSRejectUnauthorizedEnvironmentVariable := "1"
	if cfg.GetSkipSSLValidation() {
		setNodeTLSRejectUnauthorizedEnvironmentVariable = "0"
	}
	countStr := strconv.Itoa(initialInstanceCount)

	pushApp := func() error {
		var err error
		params := []string{
			"push",
			"--var", "app_name=" + appName,
			"--var", "app_domain=" + cfg.AppsDomain,
			"--var", "service_name=" + cfg.ServiceName,
			"--var", "instances=" + countStr,
			"--var", "buildpack=" + cfg.NodejsBuildpackName,
			"--var", "node_tls_reject_unauthorized=" + setNodeTLSRejectUnauthorizedEnvironmentVariable,
			"--var", "memory_mb=" + strconv.Itoa(cfg.NodeMemoryLimit),
			"-f", config.NODE_APP + "/app_manifest.yml",
			"--no-start",
		}
		params = append(params, args...)
		createApp := cf.Cf(params...).Wait(cfg.CfPushTimeoutDuration())
		if createApp.ExitCode() != 0 {
			err = fmt.Errorf("failed to push an app: %s  %s", appName, string(createApp.Err.Contents()))
			return err
		}
		return err
	}
	err := Retry(defaultRetryAttempt, defaultRetryAfter, pushApp)
	GinkgoWriter.Printf("\nfinish creating test app: %s\n", appName)
	return err
}

func CreateTestAppByName(cfg config.Config, appName string, initialInstanceCount int) {
	err := createTestApp(cfg, appName, initialInstanceCount, "-p", config.NODE_APP)
	Expect(err).ToNot(HaveOccurred())
}

func DeleteTestApp(appName string, timeout time.Duration) {
	deleteAppCmd := cf.Cf("delete", appName, "-f", "-r").Wait(timeout)
	Expect(deleteAppCmd, Exit(0), fmt.Sprintf("unable to delete app %s", deleteAppCmd.Out.Contents()))
}

func CurlAppInstance(cfg *config.Config, appName string, appInstance int, url string) string {
	appGuid, err := GetAppGuid(cfg, appName)
	Expect(err).NotTo(HaveOccurred())
	output := cfh.CurlAppWithTimeout(cfg, appName, url, 20*time.Second, "-H", fmt.Sprintf(`X-Cf-App-Instance: %s:%d`, appGuid, appInstance),
		"-f",
		"--connect-timeout", "5",
		"--max-time", "10",
		"--retry", "5",
		"--retry-delay", "0",
		"--retry-max-time", "15")
	GinkgoWriter.Printf("\n")
	return output
}

func AppSetCpuUsage(cfg *config.Config, appName string, percent int, minutes int) {
	Expect(cfh.CurlAppWithTimeout(cfg, appName, fmt.Sprintf("/cpu/%d/%d", percent, minutes), 10*time.Second)).Should(ContainSubstring(`set app cpu utilization`))
}

func AppEndCpuTest(cfg *config.Config, appName string, instance int) {
	Expect(CurlAppInstance(cfg, appName, instance, "/cpu/close")).Should(ContainSubstring(`close cpu test`))
}

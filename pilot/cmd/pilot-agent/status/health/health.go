package health

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strings"
	"time"

	meshconfig "istio.io/api/mesh/v1alpha1"
	"istio.io/istio/pkg/config/mesh"
	"istio.io/istio/pkg/util/gogoprotomarshal"
	"istio.io/pkg/env"
	"istio.io/pkg/log"
)

const (
	ISTIOD_READY    = 1
	ISTIOD_NOTREADY = 0
)

var (
	healthLog = log.RegisterScope("health", "health check log", 0)

	// 1-ready  0-not
	istiodOldStatus int = 1
	// istiodNewStatus int = 1
	mc meshconfig.MeshConfig
)

func httpProbe(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		// fmt.Printf("failed to get istiod: %v", err)
		return err
	}
	defer resp.Body.Close()
	io.Copy(ioutil.Discard, resp.Body)
	if resp.StatusCode >= 500 {
		return fmt.Errorf("failed to get istiod status: %v", resp.StatusCode)
	}
	return nil
}

func init() {
	mc = mesh.DefaultMeshConfig()
	var ProxyConfigEnv = env.RegisterStringVar(
		"PROXY_CONFIG",
		"",
		"The proxy configuration. This will be set by the injection - gateways will use file mounts.",
	).Get()

	// fmt.Println("xxxx:", mc.DefaultConfig.DiscoveryAddress)
	if ProxyConfigEnv != "" {
		var proxyConfig meshconfig.ProxyConfig

		if err := gogoprotomarshal.ApplyYAML(ProxyConfigEnv, &proxyConfig); err != nil {
			healthLog.Errorf("could not parse proxy config: %v", err)
			return
		}
		mc.DefaultConfig = &proxyConfig
	}

	if mc.DefaultConfig.DiscoveryAddress == "" {
		mc.DefaultConfig.DiscoveryAddress = "istiod.istio-system.svc:15012"
	}
}

func istiodHealth() error {
	discHost := strings.Split(mc.DefaultConfig.DiscoveryAddress, ":")[0]
	url := fmt.Sprintf("http://%s:15014", discHost)
	// healthLog.Errorf("check:%s", url)
	if err := httpProbe(url); err != nil {
		return err
	}
	return nil
}

func cleanIptables() {
	cmd := exec.Command("/bin/sh", "-c", "sudo /usr/local/bin/pilot-agent istio-clean-iptables")
	err := cmd.Start()
	if err != nil {
		healthLog.Errorf("failed to clean iptables:%s", err)
	}
}

func initIptables() {
	cmd := exec.Command("/bin/sh", "-c", "sudo /usr/local/bin/pilot-agent istio-iptables")
	err := cmd.Start()
	if err != nil {
		healthLog.Errorf("failed to init iptables:%s", err)
	}
}

func SetFailed() {
	cleanIptables()
	istiodOldStatus = ISTIOD_NOTREADY
}

func CheckIstioSystem() {
	healthLog.Debugf("start to check istio status")
	var err error
	for {
		if istiodOldStatus == ISTIOD_READY {
			down := true
			for i := 0; i < 5; i++ {
				err = istiodHealth()
				if err == nil {
					down = false
					break
				}
				healthLog.Errorf("failed to check istiod:the %d time", i)
				time.Sleep(time.Second * 2)
			}
			if down {
				healthLog.Errorf("failed to check istiod:%s, now clean iptable rules", err)
				cleanIptables()
				istiodOldStatus = ISTIOD_NOTREADY
			}
		}

		if istiodOldStatus == ISTIOD_NOTREADY {
			up := true
			for i := 0; i < 5; i++ {
				err = istiodHealth()
				if err != nil {
					up = false
					break
				}
				healthLog.Errorf("succeed to check istiod:the %d time", i)
				time.Sleep(time.Second * 2)
			}
			if up {
				healthLog.Errorf("succeed to check istiod, now rebuild iptable rules")
				initIptables()
				istiodOldStatus = ISTIOD_READY
			}
		}

		time.Sleep(time.Second * 2)
	}

	// for {
	// 	if err = istiodHealth(); err != nil {
	// 		healthLog.Errorf("failed to check istiod:%s\n", err)
	// 		if istiodOldStatus == ISTIOD_READY {
	// 			healthLog.Errorf("failed to check istiod:%s, now clean iptable rules\n", err)
	// 			cleanIptables()
	// 			istiodOldStatus = ISTIOD_NOTREADY
	// 		}
	// 	} else {
	// 		// fmt.Printf("succeed to check istiod\n")
	// 		if istiodOldStatus == ISTIOD_NOTREADY {
	// 			healthLog.Errorf("succeed to check istiod, now rebuild iptable rules\n")
	// 			initIptables()
	// 			istiodOldStatus = ISTIOD_READY
	// 		}
	// 	}

	// time.Sleep(time.Second * 10)
	// }
}

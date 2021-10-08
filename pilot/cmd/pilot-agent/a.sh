go build
#kubectl -n istio-system -c istio-proxy  cp pilot-agent envoy-test-6c959db9f8-8s5c9:/tmp/
docker build -f dockerfile -t  harbor.ushareit.me/sgt/proxyv2:1.11.0-dev .
docker push harbor.ushareit.me/sgt/proxyv2:1.11.0-dev 

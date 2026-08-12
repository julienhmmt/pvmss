package cluster

// originalFakeCloudInitConfigs returns the initial structured state used by the fake browser demo.
func originalFakeCloudInitConfigs() map[fakeCloudInitKey]CloudInitConfig {
	return map[fakeCloudInitKey]CloudInitConfig{
		{node: FakeNode01, vmid: 100}: {
			User:         FakeCloudInitUser,
			SSHKeys:      []string{"ssh-ed25519 AAAA-demo-alice@laptop"},
			IPMode:       CloudInitIPModeDHCP,
			DNSServer:    FakeCloudInitDNS,
			SearchDomain: "example.internal",
		},
		{node: FakeNode01, vmid: 101}: {
			User:         FakeCloudInitUser,
			SSHKeys:      []string{"ssh-ed25519 AAAA-demo-alice@laptop"},
			IPMode:       CloudInitIPModeStatic,
			IPAddress:    "10.0.0.42/24",
			Gateway:      "10.0.0.1",
			DNSServer:    FakeCloudInitDNS,
			SearchDomain: "example.internal",
		},
		{node: FakeNode01, vmid: 102}: {
			User:         FakeCloudInitUser,
			SSHKeys:      []string{"ssh-ed25519 AAAA-demo-alice@laptop"},
			IPMode:       CloudInitIPModeDHCP,
			DNSServer:    FakeCloudInitDNS,
			SearchDomain: "example.internal",
		},
	}
}

func cloneCloudInitConfig(config CloudInitConfig) CloudInitConfig {
	config.SSHKeys = append([]string(nil), config.SSHKeys...)
	config.Password = ""

	return config
}

package cluster

// Demo cloud-init fixture constants, defined once to avoid repeating the
// literals across the fake's seeded configs (go:S1192).
const (
	fakeSSHKey       = "ssh-ed25519 AAAA-demo-alice@laptop"
	fakeSearchDomain = "example.internal"
)

// originalFakeCloudInitConfigs returns the initial structured state used by the fake browser demo.
func originalFakeCloudInitConfigs() map[fakeCloudInitKey]CloudInitConfig {
	return map[fakeCloudInitKey]CloudInitConfig{
		{node: FakeNode01, vmid: 100}: {
			User:         FakeCloudInitUser,
			SSHKeys:      []string{fakeSSHKey},
			IPMode:       CloudInitIPModeDHCP,
			DNSServer:    FakeCloudInitDNS,
			SearchDomain: fakeSearchDomain,
		},
		{node: FakeNode01, vmid: 101}: {
			User:         FakeCloudInitUser,
			SSHKeys:      []string{fakeSSHKey},
			IPMode:       CloudInitIPModeStatic,
			IPAddress:    "10.0.0.42/24",
			Gateway:      FakeCloudInitDNS,
			DNSServer:    FakeCloudInitDNS,
			SearchDomain: fakeSearchDomain,
		},
		{node: FakeNode01, vmid: 102}: {
			User:         FakeCloudInitUser,
			SSHKeys:      []string{fakeSSHKey},
			IPMode:       CloudInitIPModeDHCP,
			DNSServer:    FakeCloudInitDNS,
			SearchDomain: fakeSearchDomain,
		},
	}
}

func cloneCloudInitConfig(config CloudInitConfig) CloudInitConfig {
	config.SSHKeys = append([]string(nil), config.SSHKeys...)
	config.Password = ""

	return config
}

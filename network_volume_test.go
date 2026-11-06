package jed

import "testing"

func TestNetworkVolumeMount(t *testing.T) {
	for _, tc := range []struct {
		name, root, prefix, service, target string
		want                                NetworkMount
	}{
		{"default_prefix", "nas:/exports/jed", "", "postgres", "/var/lib/postgresql/data", NetworkMount{"jed-postgres-var_lib_postgresql_data", "/var/lib/postgresql/data", "nas", "/exports/jed/postgres/_var_lib_postgresql_data"}},
		{"custom_prefix", "nas:/exports/jed", "team", "postgres", "/var/lib/postgresql/data", NetworkMount{"team-postgres-var_lib_postgresql_data", "/var/lib/postgresql/data", "nas", "/exports/jed/postgres/_var_lib_postgresql_data"}},
		{"trailing_slashes", "nas:/exports/jed/", "", "postgres", "/var/lib/postgresql/data/", NetworkMount{"jed-postgres-var_lib_postgresql_data", "/var/lib/postgresql/data", "nas", "/exports/jed/postgres/_var_lib_postgresql_data"}},
		{"flattened_service", "nas:/exports/jed", "", "team/app", "/data", NetworkMount{"jed-team_app-data", "/data", "nas", "/exports/jed/team_app/_data"}},
		{"ipv6", "[2001:db8::10]:/exports/jed", "", "app", "/data", NetworkMount{"jed-app-data", "/data", "2001:db8::10", "/exports/jed/app/_data"}},
		{"ipv4", "10.0.0.2:/exports/jed", "", "app", "/data", NetworkMount{"jed-app-data", "/data", "10.0.0.2", "/exports/jed/app/_data"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := networkVolumeMount(tc.root, tc.prefix, tc.service, tc.target)
			if err != nil || got != tc.want {
				t.Fatalf("got %#v, %v; want %#v", got, err, tc.want)
			}
		})
	}
}

func TestNetworkVolumeMountInvalid(t *testing.T) {
	for _, tc := range []struct {
		name, root, prefix, service, target string
	}{
		{"missing_root", "", "", "app", "/data"},
		{"missing_server", ":/exports/jed", "", "app", "/data"},
		{"missing_export", "nas:", "", "app", "/data"},
		{"relative_export", "nas:exports/jed", "", "app", "/data"},
		{"root_export", "nas:/", "", "app", "/data"},
		{"export_dot", "nas:/exports/./jed", "", "app", "/data"},
		{"export_parent", "nas:/exports/../jed", "", "app", "/data"},
		{"relative_target", "nas:/exports/jed", "", "app", "data"},
		{"root_target", "nas:/exports/jed", "", "app", "/"},
		{"empty_target", "nas:/exports/jed", "", "app", ""},
		{"target_dot", "nas:/exports/jed", "", "app", "/data/."},
		{"target_parent", "nas:/exports/jed", "", "app", "/foo/../data"},
		{"service_parent", "nas:/exports/jed", "", "foo/../app", "/data"},
		{"empty_service", "nas:/exports/jed", "", "", "/data"},
		{"server_options", "nas,rw:/exports/jed", "", "app", "/data"},
		{"server_space", "nas host:/exports/jed", "", "app", "/data"},
		{"server_port", "nas:2049:/exports/jed", "", "app", "/data"},
		{"bad_brackets", "[nas]:/exports/jed", "", "app", "/data"},
		{"unbracketed_ipv6", "2001:db8::10:/exports/jed", "", "app", "/data"},
		{"invalid_prefix", "nas:/exports/jed", ".bad", "app", "/data"},
		{"invalid_name", "nas:/exports/jed", "", "app", "/bad:name"},
		{"export_nul", "nas:/exports/\x00jed", "", "app", "/data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if mount, err := networkVolumeMount(tc.root, tc.prefix, tc.service, tc.target); err == nil {
				t.Fatalf("accepted invalid configuration: %#v", mount)
			}
		})
	}
}

func TestCloneNetworkVolumes(t *testing.T) {
	service := Service{NetworkVolumes: []string{"/data"}}
	clone := cloneService(service)
	clone.NetworkVolumes[0] = "/changed"
	if service.NetworkVolumes[0] != "/data" {
		t.Fatal("clone mutated original network declaration")
	}
}

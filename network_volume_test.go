package jed

import "testing"

func TestNetworkVolumeMount(t *testing.T) {
	for _, tc := range []struct {
		name, host, root, service, target string
		want                              NetworkMount
	}{
		{"short_name", "nas", "/exports/jed", "postgres", "/var/lib/postgresql/data", NetworkMount{"postgres_data_d2546c", "/var/lib/postgresql/data", "nas", "/exports/jed/postgres_data_d2546c"}},
		{"trailing_slashes", "nas", "/exports/jed/", "postgres", "/var/lib/postgresql/data/", NetworkMount{"postgres_data_d2546c", "/var/lib/postgresql/data", "nas", "/exports/jed/postgres_data_d2546c"}},
		{"repeated_slashes", "nas", "/exports/jed", "postgres", "//var//lib/postgresql//data", NetworkMount{"postgres_data_d2546c", "/var/lib/postgresql/data", "nas", "/exports/jed/postgres_data_d2546c"}},
		{"flattened_service", "nas", "/exports/jed", "team/app", "/data", NetworkMount{"team_app_data_848b15", "/data", "nas", "/exports/jed/team_app_data_848b15"}},
		{"flat_service_alias", "nas", "/exports/jed", "team_app", "/data", NetworkMount{"team_app_data_9ad9a5", "/data", "nas", "/exports/jed/team_app_data_9ad9a5"}},
		{"ipv6", "2001:db8::10", "/exports/jed", "app", "/data", NetworkMount{"app_data_97d481", "/data", "2001:db8::10", "/exports/jed/app_data_97d481"}},
		{"ipv4", "10.0.0.2", "/exports/jed", "app", "/data", NetworkMount{"app_data_97d481", "/data", "10.0.0.2", "/exports/jed/app_data_97d481"}},
		{"stem_24", "nas", "/exports", "app", "/dddddddddddddddddddd", NetworkMount{"app_dddddddddddddddddddd_cf4df1", "/dddddddddddddddddddd", "nas", "/exports/app_dddddddddddddddddddd_cf4df1"}},
		{"stem_25", "nas", "/exports", "app", "/ddddddddddddddddddddd", NetworkMount{"app_ddddddddddddddddddddd_40d42f", "/ddddddddddddddddddddd", "nas", "/exports/app_ddddddddddddddddddddd_40d42f"}},
		{"stem_26", "nas", "/exports", "app", "/dddddddddddddddddddddd", NetworkMount{"app_ddddddddddddddddddddd_28bb71", "/dddddddddddddddddddddd", "nas", "/exports/app_ddddddddddddddddddddd_28bb71"}},
		{"long_service", "nas", "/exports", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "/data", NetworkMount{"aaaaaaaaaaaaaaaaaaaaaaaaa_893792", "/data", "nas", "/exports/aaaaaaaaaaaaaaaaaaaaaaaaa_893792"}},
		{"same_truncated_stem", "nas", "/exports", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaab", "/data", NetworkMount{"aaaaaaaaaaaaaaaaaaaaaaaaa_bbbdbf", "/data", "nas", "/exports/aaaaaaaaaaaaaaaaaaaaaaaaa_bbbdbf"}},
		{"parent_one", "nas", "/exports", "app", "/verylongparent/data", NetworkMount{"app_data_1440fc", "/verylongparent/data", "nas", "/exports/app_data_1440fc"}},
		{"parent_two", "nas", "/exports", "app", "/anotherlongparent/data", NetworkMount{"app_data_018da3", "/anotherlongparent/data", "nas", "/exports/app_data_018da3"}},
		{"zero_padded_suffix", "nas", "/exports", "app", "/dir2/data", NetworkMount{"app_data_0c18a3", "/dir2/data", "nas", "/exports/app_data_0c18a3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := networkVolumeMount(tc.host, tc.root, tc.service, tc.target)
			if err != nil || got != tc.want {
				t.Fatalf("got %#v, %v; want %#v", got, err, tc.want)
			}
			if len(got.Name) > 32 {
				t.Fatalf("name exceeds share limit: %q", got.Name)
			}
		})
	}
}

func TestNetworkVolumeMountInvalid(t *testing.T) {
	for _, tc := range []struct {
		name, host, root, service, target string
	}{
		{"missing_root", "nas", "", "app", "/data"},
		{"missing_server", "", "/exports/jed", "app", "/data"},
		{"relative_export", "nas", "exports/jed", "app", "/data"},
		{"root_export", "nas", "/", "app", "/data"},
		{"export_dot", "nas", "/exports/./jed", "app", "/data"},
		{"export_parent", "nas", "/exports/../jed", "app", "/data"},
		{"relative_target", "nas", "/exports/jed", "app", "data"},
		{"root_target", "nas", "/exports/jed", "app", "/"},
		{"empty_target", "nas", "/exports/jed", "app", ""},
		{"target_dot", "nas", "/exports/jed", "app", "/data/."},
		{"target_parent", "nas", "/exports/jed", "app", "/foo/../data"},
		{"service_parent", "nas", "/exports/jed", "foo/../app", "/data"},
		{"empty_service", "nas", "/exports/jed", "", "/data"},
		{"server_options", "nas,rw", "/exports/jed", "app", "/data"},
		{"server_space", "nas host", "/exports/jed", "app", "/data"},
		{"server_port", "nas:2049", "/exports/jed", "app", "/data"},
		{"bad_brackets", "[nas]", "/exports/jed", "app", "/data"},
		{"bracketed_ipv6", "[2001:db8::10]", "/exports/jed", "app", "/data"},
		{"malformed_ipv6", "2001:db8:::10", "/exports/jed", "app", "/data"},
		{"invalid_name", "nas", "/exports/jed", "app", "/bad:name"},
		{"invalid_service_tail", "nas", "/exports/jed", "aaaaaaaaaaaaaaaaaaaaaaaaaa:", "/data"},
		{"invalid_target_tail", "nas", "/exports/jed", "app", "/dddddddddddddddddddddddddd:"},
		{"export_nul", "nas", "/exports/\x00jed", "app", "/data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if mount, err := networkVolumeMount(tc.host, tc.root, tc.service, tc.target); err == nil {
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

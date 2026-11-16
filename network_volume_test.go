package jed_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/logger/loggertest"
	"github.com/clarktrimble/jed/store/memo"
)

var _ = Describe("Network-volume rendering", func() {
	var (
		ctx     context.Context
		store   *memo.Store
		j       *jed.Jed
		service jed.Service
		env     jed.Env
		globals jed.Env
		spec    jed.Spec
		err     error
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = memo.New()
		service = jed.Service{
			Name:           "app",
			Image:          "app:v1",
			NetworkVolumes: []string{"/data"},
		}
		env = jed.Env{Vars: map[string]string{}}
		globals = jed.Env{
			Name: "_global",
			Vars: map[string]string{
				"NFS_HOST": "nas",
				"NFS_ROOT": "/exports/jed",
			},
		}
		j = (&jed.Config{VarsEnvName: "_global"}).New(store, loggertest.NewLoggerMock())
	})

	JustBeforeEach(func() {
		env.Name = service.Name
		Expect(store.SetService(ctx, service)).To(Succeed())
		Expect(store.SetEnv(ctx, env)).To(Succeed())
		Expect(store.SetEnv(ctx, globals)).To(Succeed())
		Expect(store.SetIntent(ctx, jed.Intent{
			Name: service.Name, Image: service.Image, Replicas: 1,
		})).To(Succeed())

		spec, err = j.Spec(ctx, service.Name)
	})

	Describe("storage identity", func() {
		When("the service declares a PostgreSQL data directory", func() {
			BeforeEach(func() {
				service.Name = "postgres"
				service.NetworkVolumes = []string{"/var/lib/postgresql/data"}
			})

			It("uses the service and last directory for readability, with a hash even for short names", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.NetworkMounts).To(ConsistOf(jed.NetworkMount{
					Name:   "postgres_data_d2546c",
					Target: "/var/lib/postgresql/data",
					Server: "nas",
					Export: "/exports/jed/postgres_data_d2546c",
				}))
				Expect(spec.Service.NetworkVolumes).To(BeNil())
			})

			When("the export root and container path have trailing slashes", func() {
				BeforeEach(func() {
					globals.Vars["NFS_ROOT"] = "/exports/jed/"
					service.NetworkVolumes = []string{"/var/lib/postgresql/data/"}
				})

				It("cleans both paths without changing the storage identity", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(spec.NetworkMounts).To(ConsistOf(jed.NetworkMount{
						Name:   "postgres_data_d2546c",
						Target: "/var/lib/postgresql/data",
						Server: "nas",
						Export: "/exports/jed/postgres_data_d2546c",
					}))
				})
			})

			When("the container path contains repeated slashes", func() {
				BeforeEach(func() {
					service.NetworkVolumes = []string{"//var//lib/postgresql//data"}
				})

				It("normalizes the full path before hashing it", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(spec.NetworkMounts).To(ConsistOf(jed.NetworkMount{
						Name:   "postgres_data_d2546c",
						Target: "/var/lib/postgresql/data",
						Server: "nas",
						Export: "/exports/jed/postgres_data_d2546c",
					}))
				})
			})
		})

		Describe("flattened service names", func() {
			When("the service name contains a path separator", func() {
				BeforeEach(func() {
					service.Name = "team/app"
				})

				It("flattens the readable name but hashes the original service identity", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(spec.NetworkMounts).To(ConsistOf(jed.NetworkMount{
						Name:   "team_app_data_848b15",
						Target: "/data",
						Server: "nas",
						Export: "/exports/jed/team_app_data_848b15",
					}))
				})
			})

			When("a different service name is already flattened", func() {
				BeforeEach(func() {
					service.Name = "team_app"
				})

				It("gives the alias a different hash despite the identical readable prefix", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(spec.NetworkMounts).To(ConsistOf(jed.NetworkMount{
						Name:   "team_app_data_9ad9a5",
						Target: "/data",
						Server: "nas",
						Export: "/exports/jed/team_app_data_9ad9a5",
					}))
				})
			})
		})

		Describe("the 32-character share-name limit", func() {
			BeforeEach(func() {
				globals.Vars["NFS_ROOT"] = "/exports"
			})

			When("the readable prefix has 24 characters", func() {
				BeforeEach(func() {
					service.NetworkVolumes = []string{"/dddddddddddddddddddd"}
				})

				It("keeps the prefix without padding it to the limit", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(spec.NetworkMounts).To(ConsistOf(jed.NetworkMount{
						Name:   "app_dddddddddddddddddddd_cf4df1",
						Target: "/dddddddddddddddddddd",
						Server: "nas",
						Export: "/exports/app_dddddddddddddddddddd_cf4df1",
					}))
					Expect(spec.NetworkMounts[0].Name).To(HaveLen(31))
				})
			})

			When("the readable prefix has exactly 25 characters", func() {
				BeforeEach(func() {
					service.NetworkVolumes = []string{"/ddddddddddddddddddddd"}
				})

				It("keeps the whole prefix and fits the separator and six-digit hash", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(spec.NetworkMounts).To(ConsistOf(jed.NetworkMount{
						Name:   "app_ddddddddddddddddddddd_40d42f",
						Target: "/ddddddddddddddddddddd",
						Server: "nas",
						Export: "/exports/app_ddddddddddddddddddddd_40d42f",
					}))
					Expect(spec.NetworkMounts[0].Name).To(HaveLen(32))
				})
			})

			When("the readable prefix exceeds 25 characters", func() {
				BeforeEach(func() {
					service.NetworkVolumes = []string{"/dddddddddddddddddddddd"}
				})

				It("shortens only the readable prefix, not the target used for hashing", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(spec.NetworkMounts).To(ConsistOf(jed.NetworkMount{
						Name:   "app_ddddddddddddddddddddd_28bb71",
						Target: "/dddddddddddddddddddddd",
						Server: "nas",
						Export: "/exports/app_ddddddddddddddddddddd_28bb71",
					}))
					Expect(spec.NetworkMounts[0].Name).To(HaveLen(32))
				})
			})

			When("the service name alone exceeds the readable-prefix limit", func() {
				BeforeEach(func() {
					service.Name = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				})

				It("bounds the share name even when truncation removes the directory name", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(spec.NetworkMounts).To(ConsistOf(jed.NetworkMount{
						Name:   "aaaaaaaaaaaaaaaaaaaaaaaaa_893792",
						Target: "/data",
						Server: "nas",
						Export: "/exports/aaaaaaaaaaaaaaaaaaaaaaaaa_893792",
					}))
					Expect(spec.NetworkMounts[0].Name).To(HaveLen(32))
				})

				When("the service name differs only beyond the truncated prefix", func() {
					BeforeEach(func() {
						service.Name = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaab"
					})

					It("uses the full service name to keep the storage identities distinct", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(spec.NetworkMounts).To(ConsistOf(jed.NetworkMount{
							Name:   "aaaaaaaaaaaaaaaaaaaaaaaaa_bbbdbf",
							Target: "/data",
							Server: "nas",
							Export: "/exports/aaaaaaaaaaaaaaaaaaaaaaaaa_bbbdbf",
						}))
						Expect(spec.NetworkMounts[0].Name).To(HaveLen(32))
					})
				})
			})
		})

		When("two targets have the same last directory under different parents", func() {
			BeforeEach(func() {
				globals.Vars["NFS_ROOT"] = "/exports"
				service.NetworkVolumes = []string{
					"/verylongparent/data",
					"/anotherlongparent/data",
				}
			})

			It("hashes the full paths so the targets do not share a Docker volume or NAS export", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.NetworkMounts).To(ConsistOf(
					jed.NetworkMount{
						Name:   "app_data_1440fc",
						Target: "/verylongparent/data",
						Server: "nas",
						Export: "/exports/app_data_1440fc",
					},
					jed.NetworkMount{
						Name:   "app_data_018da3",
						Target: "/anotherlongparent/data",
						Server: "nas",
						Export: "/exports/app_data_018da3",
					},
				))
			})
		})

		When("the identity hash begins with a zero hexadecimal digit", func() {
			BeforeEach(func() {
				globals.Vars["NFS_ROOT"] = "/exports"
				service.NetworkVolumes = []string{"/dir2/data"}
			})

			It("preserves the leading zero so the hash suffix is always six digits", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.NetworkMounts).To(ConsistOf(jed.NetworkMount{
					Name:   "app_data_0c18a3",
					Target: "/dir2/data",
					Server: "nas",
					Export: "/exports/app_data_0c18a3",
				}))
			})
		})
	})

	Describe("NFS server configuration", func() {
		When("the host is an IPv4 address", func() {
			BeforeEach(func() {
				globals.Vars["NFS_HOST"] = "10.0.0.2"
			})

			It("uses the address without changing the volume identity", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.NetworkMounts).To(ConsistOf(jed.NetworkMount{
					Name:   "app_data_97d481",
					Target: "/data",
					Server: "10.0.0.2",
					Export: "/exports/jed/app_data_97d481",
				}))
			})
		})

		When("the host is a bare IPv6 address", func() {
			BeforeEach(func() {
				globals.Vars["NFS_HOST"] = "2001:db8::10"
			})

			It("preserves the address without requiring brackets", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.NetworkMounts).To(ConsistOf(jed.NetworkMount{
					Name:   "app_data_97d481",
					Target: "/data",
					Server: "2001:db8::10",
					Export: "/exports/jed/app_data_97d481",
				}))
			})
		})

		When("the host is missing", func() {
			BeforeEach(func() { delete(globals.Vars, "NFS_HOST") })

			It("reports the required NFS_HOST setting", func() {
				Expect(err).To(MatchError(ContainSubstring("NFS_HOST")))
			})
		})

		When("the hostname exceeds 253 characters but its individual labels are valid", func() {
			BeforeEach(func() {
				label := strings.Repeat("a", 63)
				// 254 characters in total, with no label longer than 63 characters.
				globals.Vars["NFS_HOST"] = strings.Join([]string{
					label, label, label, strings.Repeat("a", 62),
				}, ".")
			})

			It("rejects the overlong hostname independently of the label-length limit", func() {
				Expect(err).To(MatchError(ContainSubstring("NFS_HOST")))
			})
		})

		When("the host includes mount options", func() {
			BeforeEach(func() { globals.Vars["NFS_HOST"] = "nas,rw" })

			It("rejects option injection through the server address", func() {
				Expect(err).To(MatchError(ContainSubstring("NFS_HOST")))
			})
		})

		When("the host contains whitespace", func() {
			BeforeEach(func() { globals.Vars["NFS_HOST"] = "nas host" })

			It("rejects the malformed server address", func() {
				Expect(err).To(MatchError(ContainSubstring("NFS_HOST")))
			})
		})

		When("the host includes a port suffix", func() {
			BeforeEach(func() { globals.Vars["NFS_HOST"] = "nas:2049" })

			It("rejects the port rather than treating it as part of the hostname", func() {
				Expect(err).To(MatchError(ContainSubstring("NFS_HOST")))
			})
		})

		When("a hostname is enclosed in brackets", func() {
			BeforeEach(func() { globals.Vars["NFS_HOST"] = "[nas]" })

			It("rejects bracket syntax instead of silently stripping it", func() {
				Expect(err).To(MatchError(ContainSubstring("NFS_HOST")))
			})
		})

		When("an IPv6 address is enclosed in brackets", func() {
			BeforeEach(func() { globals.Vars["NFS_HOST"] = "[2001:db8::10]" })

			It("requires the bare IPv6 form", func() {
				Expect(err).To(MatchError(ContainSubstring("NFS_HOST")))
			})
		})

		When("an IPv6 address is malformed", func() {
			BeforeEach(func() { globals.Vars["NFS_HOST"] = "2001:db8:::10" })

			It("rejects the invalid address", func() {
				Expect(err).To(MatchError(ContainSubstring("NFS_HOST")))
			})
		})
	})

	Describe("NFS export-root validation", func() {
		When("the root is missing", func() {
			BeforeEach(func() { delete(globals.Vars, "NFS_ROOT") })

			It("reports the required NFS_ROOT setting", func() {
				Expect(err).To(MatchError(ContainSubstring("NFS_ROOT")))
			})
		})

		When("the root is relative", func() {
			BeforeEach(func() { globals.Vars["NFS_ROOT"] = "exports/jed" })

			It("requires an absolute export path", func() {
				Expect(err).To(MatchError(ContainSubstring("NFS_ROOT")))
			})
		})

		When("the root is the filesystem root", func() {
			BeforeEach(func() { globals.Vars["NFS_ROOT"] = "/" })

			It("requires a directory below the filesystem root", func() {
				Expect(err).To(MatchError(ContainSubstring("NFS_ROOT")))
			})
		})

		When("the root contains a current-directory segment", func() {
			BeforeEach(func() { globals.Vars["NFS_ROOT"] = "/exports/./jed" })

			It("rejects the dot segment instead of normalizing it away", func() {
				Expect(err).To(MatchError(ContainSubstring("NFS_ROOT")))
			})
		})

		When("the root contains a parent-directory segment", func() {
			BeforeEach(func() { globals.Vars["NFS_ROOT"] = "/exports/../jed" })

			It("rejects traversal even if cleaning would produce an absolute path", func() {
				Expect(err).To(MatchError(ContainSubstring("NFS_ROOT")))
			})
		})

		When("the root contains a NUL byte", func() {
			BeforeEach(func() { globals.Vars["NFS_ROOT"] = "/exports/\x00jed" })

			It("rejects an export path that cannot be used by the mount system", func() {
				Expect(err).To(MatchError(ContainSubstring("NFS_ROOT")))
			})
		})
	})

	Describe("container-path validation", func() {
		When("a service with network storage has a local-volume template that expands to traversal", func() {
			BeforeEach(func() {
				service.NetworkVolumes = []string{"/data"}
				service.LocalVolumes = []string{"{{CACHE_PATH}}"}
				env.Vars["CACHE_PATH"] = "/cache/../data"
			})

			It("rejects the invalid local target before requiring LOCAL_ROOT", func() {
				Expect(err).To(MatchError(And(
					ContainSubstring("local volume"),
					ContainSubstring("cannot contain . or .."),
				)))
			})
		})

		When("the target is relative", func() {
			BeforeEach(func() { service.NetworkVolumes = []string{"data"} })

			It("requires an absolute container path", func() {
				Expect(err).To(MatchError(ContainSubstring("network volume")))
			})
		})

		When("the target is the container root", func() {
			BeforeEach(func() { service.NetworkVolumes = []string{"/"} })

			It("requires a directory below the container root", func() {
				Expect(err).To(MatchError(ContainSubstring("network volume")))
			})
		})

		When("the target is empty", func() {
			BeforeEach(func() { service.NetworkVolumes = []string{""} })

			It("does not interpret the empty declaration as a missing volume", func() {
				Expect(err).To(MatchError(ContainSubstring("network volume")))
			})
		})

		When("the target contains a current-directory segment", func() {
			BeforeEach(func() { service.NetworkVolumes = []string{"/data/."} })

			It("rejects the dot segment instead of assigning a normalized storage identity", func() {
				Expect(err).To(MatchError(ContainSubstring("network volume")))
			})
		})

		When("the target contains a parent-directory segment", func() {
			BeforeEach(func() { service.NetworkVolumes = []string{"/foo/../data"} })

			It("rejects traversal instead of silently mounting a different target", func() {
				Expect(err).To(MatchError(ContainSubstring("network volume")))
			})
		})
	})

	Describe("service-name and share-name validation", func() {
		When("the service name contains a parent-directory segment", func() {
			BeforeEach(func() { service.Name = "foo/../app" })

			It("rejects traversal instead of flattening it into a share name", func() {
				Expect(err).To(MatchError(ContainSubstring("service name")))
			})
		})

		When("the service name is empty", func() {
			BeforeEach(func() { service.Name = "" })

			It("requires a service identity before deriving storage", func() {
				Expect(err).To(MatchError(ContainSubstring("name is required")))
			})
		})

		When("the last directory contains a character invalid in a Docker volume name", func() {
			BeforeEach(func() { service.NetworkVolumes = []string{"/bad:name"} })

			It("rejects the derived name before returning a deployable spec", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid network volume name")))
			})
		})

		When("an invalid service-name character would be removed by truncation", func() {
			BeforeEach(func() { service.Name = "aaaaaaaaaaaaaaaaaaaaaaaaaa:" })

			It("validates the entire readable prefix before shortening it", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid network volume name")))
			})
		})

		When("an invalid directory-name character would be removed by truncation", func() {
			BeforeEach(func() {
				service.NetworkVolumes = []string{"/dddddddddddddddddddddddddd:"}
			})

			It("does not hide the invalid character behind the share-name limit", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid network volume name")))
			})
		})
	})

	When("the network target is supplied by the service environment", func() {
		BeforeEach(func() {
			service.NetworkVolumes = []string{"{{VOL_PATH}}"}
			env.Vars["VOL_PATH"] = "/data"
		})

		It("leaves stored declarations templated after rendering", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.NetworkMounts).To(ConsistOf(jed.NetworkMount{
				Name:   "app_data_97d481",
				Target: "/data",
				Server: "nas",
				Export: "/exports/jed/app_data_97d481",
			}))
			Expect(spec.Service.NetworkVolumes).To(BeNil())

			stored, getErr := store.GetService(ctx, "app", "app:v1")
			Expect(getErr).NotTo(HaveOccurred())
			Expect(stored.NetworkVolumes).To(Equal([]string{"{{VOL_PATH}}"}))
		})

		It("uses a proposed target without changing later renders of stored state", func() {
			Expect(err).NotTo(HaveOccurred())
			proposal := jed.Env{
				Name: "app",
				Vars: map[string]string{"VOL_PATH": "/other/data"},
			}

			provisional, renderErr := j.ProvisionalSpec(ctx, spec.Intent, proposal)
			Expect(renderErr).NotTo(HaveOccurred())
			Expect(provisional.NetworkMounts).To(ConsistOf(jed.NetworkMount{
				Name:   "app_data_34370a",
				Target: "/other/data",
				Server: "nas",
				Export: "/exports/jed/app_data_34370a",
			}))

			again, renderErr := j.Spec(ctx, "app")
			Expect(renderErr).NotTo(HaveOccurred())
			Expect(again.NetworkMounts).To(ConsistOf(jed.NetworkMount{
				Name:   "app_data_97d481",
				Target: "/data",
				Server: "nas",
				Export: "/exports/jed/app_data_97d481",
			}))
			stored, getErr := store.GetService(ctx, "app", "app:v1")
			Expect(getErr).NotTo(HaveOccurred())
			Expect(stored.NetworkVolumes).To(Equal([]string{"{{VOL_PATH}}"}))
			storedEnv, getErr := store.GetEnv(ctx, "app")
			Expect(getErr).NotTo(HaveOccurred())
			Expect(storedEnv.Vars).To(Equal(map[string]string{"VOL_PATH": "/data"}))
		})

		It("returns mounts that can be changed without affecting another render", func() {
			Expect(err).NotTo(HaveOccurred())
			second, renderErr := j.Spec(ctx, "app")
			Expect(renderErr).NotTo(HaveOccurred())

			spec.NetworkMounts[0].Name = "changed"

			Expect(second.NetworkMounts).To(ConsistOf(jed.NetworkMount{
				Name:   "app_data_97d481",
				Target: "/data",
				Server: "nas",
				Export: "/exports/jed/app_data_97d481",
			}))
		})
	})
})

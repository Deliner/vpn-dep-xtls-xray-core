package release

func archiveName(arch string) string { return "cronet-linux-" + arch + ".zip" }

func memberSources() map[string]string {
	return map[string]string{
		"libcronet.so": "build/bin/libcronet.so", "source.tsv": "build/source.tsv",
		"build-provenance.txt": "build/provenance.txt", "build-evidence.sha256": "build/evidence.sha256",
		"native-evidence.sha256": "native/evidence.sha256", "native-identity.txt": "native/identity.txt",
		"naive-evidence.sha256": "native/naive/evidence.sha256", "zlib-evidence.sha256": "native/zlib-window/evidence.sha256",
	}
}

package catalog

// troubleshootingExplicit maps the troubleshooting binaries onto their
// per-command metadata tables (one file per product so the products can be
// developed independently).
var troubleshootingExplicit = map[string]func(string) (explicitMeta, bool){
	"nexus":  nexusExplicit,
	"splunk": splunkExplicit,
	"appd":   appdExplicit,
	"pgsql":  pgsqlExplicit,
}

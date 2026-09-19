package splunk

import "testing"

func TestBlockedCommand(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		blocked string
	}{
		{"plain terms", "index=main error", ""},
		{"explicit search command", "search index=main error | stats count by host", ""},
		{"generating command", "| tstats count where index=main by host", ""},
		{"head and table", "index=main | head 10 | table _time host", ""},
		{"pipe inside quoted string", `index=main "| delete"`, ""},
		{"escaped quote inside string", `index=main msg="say \"hi\" | delete" | head 5`, ""},
		{"search term delete", "search delete", ""},
		{"inputlookup allowed", "| inputlookup users.csv | head 5", ""},
		{"rest allowed", "| rest /services/server/info", ""},
		{"map with safe search", `index=main | map search="search index=other host=$host$ | head 1"`, ""},
		{"empty", "", ""},
		// Splunk strips ```...``` before running the search, so a comment where
		// the command word belongs must not become the command word.
		{"comment hides delete", "index=main | ```c``` delete", "delete"},
		{"comment hides collect", "search index=x | ```note``` collect index=y", "collect"},
		{"comment hides outputlookup, no spaces", "index=x |```x```outputlookup z.csv", "outputlookup"},
		{"comment inside map search", "index=x | map search=\"search index=x | ```c``` collect index=y\"", "collect"},
		{"comment between safe commands stays allowed", "index=main | ```why``` head 10", ""},
		// A macro is expanded by Splunk, not here, so its body is unknowable.
		{"macro", "index=x | `my_macro`", MacroToken},
		{"macro with arguments", "index=x | `m(1)` | head 5", MacroToken},
		{"unterminated comment run trips the macro check", "index=x | ``` delete", MacroToken},
		{"dump", "index=main | dump basefilename=x", "dump"},
		{"delete", "index=main | delete", "delete"},
		{"delete no spaces", "index=main|delete", "delete"},
		{"delete extra spaces", "index=main |   delete", "delete"},
		{"delete uppercase", "index=main | DELETE", "delete"},
		{"leading delete", "delete", "delete"},
		{"outputlookup generating", "| outputlookup bad.csv", "outputlookup"},
		{"outputcsv", "index=main | outputcsv dump", "outputcsv"},
		{"outputtext", "index=main | outputtext", "outputtext"},
		{"collect", "index=main | collect index=summary", "collect"},
		{"mcollect", "index=main | mcollect index=metrics", "mcollect"},
		{"meventcollect", "index=main | meventcollect index=metrics", "meventcollect"},
		{"sendemail", "index=main | sendemail to=ops@example.test", "sendemail"},
		{"sendalert", "index=main | sendalert webhook", "sendalert"},
		{"script", "| script python export", "script"},
		{"runshellscript", "index=main | runshellscript run.sh", "runshellscript"},
		{"tscollect", "index=main | tscollect namespace=x", "tscollect"},
		{"summaryindex", "index=main | summaryindex", "summaryindex"},
		{"blocked after several stages", "index=main | head 10 | eval x=1 | outputlookup out.csv", "outputlookup"},
		{"blocked inside subsearch", "index=main [search index=x | outputlookup y.csv]", "outputlookup"},
		{"blocked inside append", "index=main | append [search index=x | collect index=y]", "collect"},
		{"unbalanced quote does not hide stage", `index=main " | delete`, "delete"},
		{"single quotes do not hide stage", "index=main can't | delete", "delete"},
		{"map with blocked search", `index=main | map search="search index=main | delete"`, "delete"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, blocked := BlockedCommand(tc.query)
			if blocked != (tc.blocked != "") || cmd != tc.blocked {
				t.Fatalf("BlockedCommand(%q) = (%q, %v), want (%q, %v)", tc.query, cmd, blocked, tc.blocked, tc.blocked != "")
			}
		})
	}
}

func TestBlockedCommandsListMatchesTable(t *testing.T) {
	list := BlockedCommands()
	if len(list) != len(blockedCommands) {
		t.Fatalf("list has %d entries, table has %d", len(list), len(blockedCommands))
	}
	for _, name := range list {
		if !blockedCommands[name] {
			t.Fatalf("%s listed but not blocked", name)
		}
	}
}

func TestPrepareSearch(t *testing.T) {
	cases := []struct {
		name, query, index, want string
	}{
		{"adds search prefix", "error | head 5", "", "search error | head 5"},
		{"keeps search prefix", "search error", "", "search error"},
		{"keeps uppercase search prefix", "SEARCH error", "", "SEARCH error"},
		{"generating command untouched", "| tstats count where index=main", "main", "| tstats count where index=main"},
		{"injects default index", "error | head 5", "main", "search index=main error | head 5"},
		{"injects after search keyword", "search error", "main", "search index=main error"},
		{"does not inject when index present", "index=other error", "main", "search index=other error"},
		{"does not inject when index spaced", "index = other error", "main", "search index = other error"},
		{"does not inject when index IN", "index IN (a, b) error", "main", "search index IN (a, b) error"},
		{"trims whitespace", "  error  ", "", "search error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PrepareSearch(tc.query, tc.index); got != tc.want {
				t.Fatalf("PrepareSearch(%q, %q) = %q, want %q", tc.query, tc.index, got, tc.want)
			}
		})
	}
}

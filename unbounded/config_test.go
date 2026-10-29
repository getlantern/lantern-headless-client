package unbounded

import "testing"

func TestParseRate(t *testing.T) {
	cases := []struct {
		in   string
		want Rate
	}{
		{"", 0},
		{"0", 0},
		{"5mbit", 625_000},
		{"5Mbit/s", 625_000},
		{"20Mbps", 2_500_000},
		{"8kb", 1000},
		{"500KB/s", 500_000},
		{"500KB", 500_000},
		{"1.5MiB/s", 1_572_864},
		{"2MB", 2_000_000},
		{"1gbit", 125_000_000},
		{"100B/s", 100},
	}
	for _, c := range cases {
		got, err := ParseRate(c.in)
		if err != nil {
			t.Errorf("ParseRate(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseRate(%q) = %d, want %d", c.in, got, c.want)
		}
	}
	for _, bad := range []string{"fast", "5", "5 mb/h", "-1mbit", "1bit"} {
		if _, err := ParseRate(bad); err == nil {
			t.Errorf("ParseRate(%q) accepted", bad)
		}
	}
}

func TestParseDataCap(t *testing.T) {
	cases := []struct {
		in   string
		want DataCap
	}{
		{"", DataCap{}},
		{"0", DataCap{}},
		{"50GB", DataCap{Bytes: 50_000_000_000, Period: PeriodDay}},
		{"50GB/day", DataCap{Bytes: 50_000_000_000, Period: PeriodDay}},
		{"200GiB/week", DataCap{Bytes: 200 << 30, Period: PeriodWeek}},
		{"1TB/month", DataCap{Bytes: 1_000_000_000_000, Period: PeriodMonth}},
		{"1.5 TiB / month", DataCap{Bytes: 3 << 39, Period: PeriodMonth}},
	}
	for _, c := range cases {
		got, err := ParseDataCap(c.in)
		if err != nil {
			t.Errorf("ParseDataCap(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseDataCap(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
	for _, bad := range []string{"lots", "50GB/hour", "50GB/"} {
		if _, err := ParseDataCap(bad); err == nil {
			t.Errorf("ParseDataCap(%q) accepted", bad)
		}
	}
}

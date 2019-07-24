package Remon

type commChan struct {
	cmd   string
	body  string
	media *commMedia
}

type commMedia struct {
	audio    bool
	data     *[]byte
	ts       uint64
	duration float64
}

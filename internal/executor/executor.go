package executor

import (
	"bufio"
	"fmt"
	"github.com/projectdiscovery/gologger"
	"os"
	"sync"

	"github.com/logrusorgru/aurora"
	"github.com/projectdiscovery/retryabledns"
	"github.com/pwnesia/dnstake/internal/errors"
	"github.com/pwnesia/dnstake/internal/option"
	"github.com/pwnesia/dnstake/pkg/dnstake"
	"github.com/pwnesia/dnstake/pkg/fingerprint"
)

var mu sync.Mutex

// WriteToFile writes output data into specified file.
func WriteToFile(data, outputFile string) {

	mu.Lock()
	defer mu.Unlock()

	file, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		gologger.Error().Msg(err.Error())
	}

	wrt := bufio.NewWriter(file)

	_, err = wrt.WriteString(data + "\n")
	if err != nil {
		gologger.Error().Msg(err.Error())
	}

	wrt.Flush()
	file.Close()
}

// New to execute target hostname
func New(opt *option.Options, hostname string) {
	var out = ""

	vuln, DNS, err := exec(hostname)
	if err != nil {
		out += fmt.Sprintf("[%s] %s: %s", aurora.Red("ERR"), hostname, err.Error())
	}

	if vuln {
		if !opt.Silent {
			out += fmt.Sprintf("[%s] ", aurora.Green("VLN"))
		}

		out += hostname

		if !opt.Silent {
			out += fmt.Sprintf(" (%s)", aurora.Cyan(DNS.Provider))
		}

		if !opt.Silent {
			for _, status := range DNS.Status {
				switch status {
				case 2:
					out += fmt.Sprintf(" (%s)", aurora.Magenta("Edge Case"))
				case 3:
					out += fmt.Sprintf(" (%s)", aurora.Yellow("$"))
				}
			}
		}

	}

	if opt.Output != "" {
		WriteToFile(out, opt.Output)
	}

	fmt.Println(out)

}

func exec(hostname string) (bool, fingerprint.DNS, error) {
	var (
		vuln bool
		DNS  = fingerprint.DNS{}
	)

	client := retryabledns.New([]string{"8.8.8.8:53", "1.1.1.1:53"}, 3)

	q1, err := dnstake.Resolve(client, hostname, 2)
	if err != nil {
		return vuln, DNS, fmt.Errorf("%s", errors.ErrResolve)
	}

	if len(q1.NS) < 1 {
		return vuln, DNS, fmt.Errorf("%s", errors.ErrNoNSRec)
	}

	fgp, rec, err := fingerprint.Check(q1.NS)
	if err != nil {
		return vuln, fgp, fmt.Errorf("%s (%s)", errors.ErrPattern, err.Error())
	}

	if rec == "" {
		return false, fgp, fmt.Errorf("%s", errors.ErrFinger)
	}

	if _, m := find(fgp.Status, 0); m {
		return vuln, DNS, fmt.Errorf("%s", errors.ErrNotVuln)
	}

	q2, err := dnstake.Resolve(client, rec, 1)
	if err != nil {
		return vuln, DNS, fmt.Errorf("%s (%s)", errors.ErrResolve, rec)
	}

	vuln, err = dnstake.Verify(q2.A, hostname)
	if err != nil {
		return vuln, DNS, fmt.Errorf("%s (%s)", errors.ErrVerify, err.Error())
	}

	if vuln {
		return vuln, fgp, nil
	}

	return false, fgp, fmt.Errorf("%s", errors.ErrUnknown)
}

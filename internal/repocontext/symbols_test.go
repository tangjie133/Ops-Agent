package repocontext

import "testing"

func TestExtractSearchTerms_RTCIssue(t *testing.T) {
	text := `I can't get the 1Hz square wave output to work for this RTC.
The only way I have managed to get the output working on the INT pin is to change your code for
DFRobot_SD3031::enableFrequency(eFrequency_t fr) to this,
void DFRobot_SD3031::enableFrequency(eFrequency_t fr)
{
readReg(SD3031_REG_CTR2, &reg2, 1);
readReg(SD3031_REG_CTR3, &reg3, 1);
reg2 = 0xEF;
reg3 = reg3 | fr;
writeReg(SD3031_REG_CTR2, &reg2, 1);
}`
	terms := ExtractSearchTerms(text)
	want := map[string]bool{
		"DFRobot_SD3031::enableFrequency": true,
		"DFRobot_SD3031":                  true,
		"enableFrequency":                 true,
		"SD3031_REG_CTR2":                 true,
		"SD3031_REG_CTR3":                 true,
		"eFrequency_t":                    true,
	}
	for k := range want {
		found := false
		for _, term := range terms {
			if term == k {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing term %q in %v", k, terms)
		}
	}
}

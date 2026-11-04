package kyc

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// KYC_COMPARE=1 prints every certificate, bill and ID field next to the registry value for all
// 12 applicants (the field-by-field comparison behind docs/kyc-findings.md).
func TestCompareAllFields(t *testing.T) {
	if os.Getenv("KYC_COMPARE") == "" {
		t.Skip("set KYC_COMPARE=1")
	}
	ref, want := loadExpectations(t)
	reg := testRegistry(t)
	ids := make([]string, 0, len(want))
	for id := range want {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	same := func(a, b string) string {
		if a == "" || b == "" {
			return "n/a"
		}
		if SameName(a, b, 0.85) || Similarity(addrKey(a), addrKey(b)) >= 0.9 {
			return "same"
		}
		return "DIFFERS"
	}
	fmt.Println("| ID | Field | Document | Registry | |")
	fmt.Println("|---|---|---|---|---|")
	for _, id := range ids {
		res, err := Run(context.Background(), testOCR(t), reg, DefaultPolicy(ref), specimenDocs(t, id))
		if err != nil {
			t.Fatal(err)
		}
		f := func(d, k string) string { return res.Documents[d].Extracted.Fields[k] }
		row := func(field, doc, regv string) {
			fmt.Printf("| %s | %s | %s | %s | %s |\n", id, field, doc, regv, same(doc, regv))
		}
		g, u := res.Verified.GSTINRecord, res.Verified.UdyamRecord
		if g != nil {
			owner, biz := splitLegalName(f(DocGST, FLegalName))
			rOwner, rBiz := splitLegalName(g.LegalName)
			row("GST legal name (business)", biz, rBiz)
			row("GST legal name (proprietor)", owner, rOwner)
			row("GST trade name", f(DocGST, FTradeName), g.TradeName)
			row("GST constitution", f(DocGST, FConstitution), g.Constitution)
			row("GST address", f(DocGST, FAddress), g.Address)
			row("GST date of liability vs registration date", f(DocGST, FLiability), g.RegistrationDate)
			row("GST status", "", g.Status)
			row("Bill billed to", f(DocAddress, FBilledTo), g.TradeName)
			row("Bill address", f(DocAddress, FAddress), g.Address)
			row("Signatory name vs proprietor", f(DocSignatory, FName), rOwner)
			row("Signatory business", f(DocSignatory, FBusiness), g.TradeName)
		}
		if u != nil {
			row("Udyam enterprise name", f(DocUdyam, FEnterprise), u.EnterpriseName)
			row("Udyam major activity", f(DocUdyam, FActivity), u.MajorActivity)
			row("Udyam city", f(DocUdyam, FCity), u.City)
			row("Udyam state", f(DocUdyam, FState), u.State)
			row("Udyam date of registration", f(DocUdyam, FUdyamDate), u.DateOfRegistration)
			row("Udyam type / NIC", "", u.EnterpriseType+" / "+u.NICDescription)
		}
		fmt.Printf("| %s | Signatory designation / ID valid till / bill date | %s / %s / %s | | |\n", id,
			f(DocSignatory, FDesignation), f(DocSignatory, FValidTill), f(DocAddress, FBillDate))
		fmt.Printf("| %s | **route** | %s %s | gstin %s, udyam %s | |\n", id, res.Route, strings.Join(res.Codes(), ","),
			res.Verified.GSTINMatchType, res.Verified.UdyamMatchType)
	}
}

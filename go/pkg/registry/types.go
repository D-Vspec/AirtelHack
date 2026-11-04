package registry

// GSTIN is one row of registry.gstin, as served by GET /gstin/{id}.
type GSTIN struct {
	GSTIN            string `json:"gstin" db:"gstin"`
	LegalName        string `json:"legalName" db:"legal_name"`
	TradeName        string `json:"tradeName" db:"trade_name"`
	Constitution     string `json:"constitution" db:"constitution"`
	Address          string `json:"address" db:"principal_place_of_business"`
	State            string `json:"state" db:"state"`
	RegistrationDate string `json:"registrationDate" db:"registration_date"`
	Status           string `json:"status" db:"status"`
}

// Udyam is one row of registry.udyam, as served by GET /udyam/{id}.
type Udyam struct {
	UdyamNumber        string `json:"udyamNumber" db:"udyam_number"`
	EnterpriseName     string `json:"enterpriseName" db:"enterprise_name"`
	EnterpriseType     string `json:"enterpriseType" db:"enterprise_type"`
	MajorActivity      string `json:"majorActivity" db:"major_activity"`
	NICDescription     string `json:"nicDescription" db:"nic_description"`
	State              string `json:"state" db:"state"`
	City               string `json:"city" db:"city"`
	DateOfRegistration string `json:"dateOfRegistration" db:"date_of_registration"`
}

// GSTINMatch is the GET /gstin/{id} response body.
type GSTINMatch struct {
	Match[GSTIN]
	ChecksumValid bool `json:"checksumValid"`
}

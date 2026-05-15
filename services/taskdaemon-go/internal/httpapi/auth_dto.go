package httpapi

type initializeAdminRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type initializeAdminResponse struct {
	AdminID   int    `json:"adminId"`
	Username  string `json:"username"`
	CSRFToken string `json:"csrfToken"`
}

type authStatusResponse struct {
	Initialized   bool               `json:"initialized"`
	Authenticated bool               `json:"authenticated"`
	Admin         *authAdminResponse `json:"admin,omitempty"`
}

type authAdminResponse struct {
	AdminID  int    `json:"adminId"`
	Username string `json:"username"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	AdminID   int    `json:"adminId"`
	Username  string `json:"username"`
	CSRFToken string `json:"csrfToken"`
}

type meResponse struct {
	AdminID  int    `json:"adminId"`
	Username string `json:"username"`
}

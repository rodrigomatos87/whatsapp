package models

// Conta é uma conta de WhatsApp pareada neste servidor (modo avançado do
// Ravi permite mais de uma; a "principal" é a herdada pelos ambientes).
type Conta struct {
	Numero    string `json:"numero"`
	JID       string `json:"jid"`
	Nome      string `json:"nome"`
	Conectado bool   `json:"conectado"`
	Principal bool   `json:"principal"`
}

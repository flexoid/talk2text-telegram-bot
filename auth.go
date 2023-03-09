package main

// Simple PIN-based authentication.
// Important: this is not secure and has potential vulnerabilities and memory leak.
// Using it only for demo purposes.
type PinAuthenticator struct {
	pin   string
	users map[int64]bool
}

func NewPinAuthenticator(pin string) PinAuthenticator {
	return PinAuthenticator{
		pin:   pin,
		users: make(map[int64]bool),
	}
}

func (p PinAuthenticator) CheckAuth(userID int64) bool {
	return p.users[userID]
}

func (p PinAuthenticator) Authenticate(userID int64, text string) bool {
	if p.pin == text {
		p.users[userID] = true
		return true
	}

	return false
}

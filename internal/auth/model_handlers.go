package auth

// UserResponse is the API `User` shape: camelCase, no identity subject or
// timestamps.
type UserResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

// MeResponse is /api/v1/me minus `settings` and `apiKey`: neither table exists
// until M2.
type MeResponse struct {
	User UserResponse `json:"user"`
}

func newUserResponse(u *User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		AvatarURL: u.AvatarURL,
	}
}

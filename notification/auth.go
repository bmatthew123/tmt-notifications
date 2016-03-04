package notification

import (
	"errors"
	"fmt"
	jwt "github.com/dgrijalva/jwt-go"
	"io/ioutil"
	"net/http"
	"os"
)

// Checks the given Authorization header for an encoded
// JWT and verifies it is from a valid sender. Retrieves
// the employee NetId and area Guid and stores in the
// context. Intended to be used as middleware.
func Authorize(req *http.Request) (User, error) {

	// Parse JWT from Authorization header
	var tokenString string
	tokenString = req.Header.Get("Authorization")
	if len(tokenString) < 1 {
		if token := req.URL.Query().Get("auth"); len(token) != 0 {
			tokenString = token
		} else {
			return User{}, errors.New("No authorization header present")
		}
	}

	// Find the directory with the RSA keys
	dir := os.Getenv("KEYS_DIRECTORY")
	if dir == "" {
		dir = "./keys" // If none given, use the keys directory within the current directory
	}

	// Loop over available public key files
	files, _ := ioutil.ReadDir(dir)
	for _, f := range files {
		name := f.Name()
		// Only look at files with extension .pub
		if name[len(name)-4:] == ".pub" {
			// Once a public is found, decode and try to validate
			// See documentation on the JWT library for how this works
			// The function passed in is a function that looks up the key
			decoded, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				key, err := ioutil.ReadFile(dir + "/" + name) // Read in public key
				if err != nil {
					return nil, err
				}
				return jwt.ParseRSAPublicKeyFromPEM(key)
			})

			// If there was no error in parsing the key and decoding the token
			// and if the token is valid store employee and area guids
			if err == nil && decoded.Valid {
				user := User{}
				user.NetId = fmt.Sprintf("%v", decoded.Claims["employee"])
				user.Area = fmt.Sprintf("%v", decoded.Claims["area"])
				return user, nil
			}
		}
	}

	// No public key was able to decode and validate the JWT
	return User{}, errors.New("You are not authorized to make this request")
}

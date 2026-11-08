package websiteapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	jwtmiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/form3tech-oss/jwt-go"
)

type Jwks struct {
	Keys []JSONWebKeys `json:"keys"`
}

type JSONWebKeys struct {
	Kty string   `json:"kty"`
	Kid string   `json:"kid"`
	Use string   `json:"use"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5c []string `json:"x5c"`
}

var jwtMiddleware *jwtmiddleware.JWTMiddleware

func Auth0JwtMiddleware() *jwtmiddleware.JWTMiddleware {
	if jwtMiddleware == nil {
		jwtMiddleware = jwtmiddleware.New(jwtmiddleware.Options{
			ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
				// Verify 'aud' claim
				aud := "https://api.ttnmapper.org"
				checkAud := token.Claims.(jwt.MapClaims).VerifyAudience(aud, false)
				if !checkAud {
					return token, errors.New("invalid audience")
				}
				// Verify 'iss' claim
				iss := "https://auth.ttnmapper.org/"
				checkIss := token.Claims.(jwt.MapClaims).VerifyIssuer(iss, false)
				if !checkIss {
					return token, errors.New("invalid issuer")
				}

				cert, err := getPemCert(token)
				if err != nil {
					panic(err.Error())
				}

				result, _ := jwt.ParseRSAPublicKeyFromPEM([]byte(cert))
				return result, nil
			},
			SigningMethod: jwt.SigningMethodRS256,
		})
	}
	return jwtMiddleware
}

func getPemCert(token *jwt.Token) (string, error) {
	cert := ""
	resp, err := http.Get("https://auth.ebug.co.za/.well-known/jwks.json")

	if err != nil {
		return cert, err
	}
	defer resp.Body.Close()

	var jwks = Jwks{}
	err = json.NewDecoder(resp.Body).Decode(&jwks)

	if err != nil {
		return cert, err
	}

	for k, _ := range jwks.Keys {
		if token.Header["kid"] == jwks.Keys[k].Kid {
			cert = "-----BEGIN CERTIFICATE-----\n" + jwks.Keys[k].X5c[0] + "\n-----END CERTIFICATE-----"
		}
	}

	if cert == "" {
		err := errors.New("Unable to find appropriate key.")
		return cert, err
	}

	return cert, nil
}

func GetUseridFromRequest(r *http.Request) (int, error) {
	user := r.Context().Value("user")
	//for k, v := range user.(*jwt.Token).Claims.(jwt.MapClaims) {
	//	log.Printf("%s :\t%#v\n", k, v)
	//}
	userIdString, ok := user.(*jwt.Token).Claims.(jwt.MapClaims)["sub"].(string) // sub :	"auth0|1211"
	if !ok {
		return 0, errors.New("can't get userid from request context")
	}
	userIdString = strings.TrimPrefix(userIdString, "auth0|")

	userId, err := strconv.Atoi(userIdString)
	if err != nil {
		// Return error
		return 0, err
	}

	return userId, nil
}

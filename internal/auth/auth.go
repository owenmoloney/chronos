package auth

import(
	"github.com/golang-jwt/jwt/v5"
	"time"
	"errors"
)
type Claims struct{
	TenantId 	int64 `json:"tenant_id"`
	jwt.RegisteredClaims
}


func IssueToken(secret string, tenantID int64, ttl time.Duration) (token string, expiredAt time.Time, err error){
	expiredAt = time.Now().UTC().Add(ttl)

	claims := Claims{
		TenantId: tenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiredAt),
			IssuedAt: jwt.NewNumericDate(time.Now().UTC()),
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	token, err = tok.SignedString([]byte(secret))

	if err != nil{
		return "", time.Time{}, err
	}

	return token, expiredAt, nil
}

func ParseToken(secret string, tokenString string) (Claims, error){
	
	claimsPtr := &Claims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claimsPtr,
		func (t *jwt.Token) (any, error){
			if t.Method.Alg() != jwt.SigningMethodHS256.Alg(){
				return nil, errors.New("Unexpected Singing Method")
			}
		return []byte(secret), nil
		},
	)

	if err != nil{
		return Claims{}, err
	}

	if !token.Valid{
		return Claims{}, errors.New("Invalid Token")
	}


	return *claimsPtr, nil
}
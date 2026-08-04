package execute

import(
	"testing"
)

func TestSafeURL(t *testing.T){

  cases := []struct{
    name    string
    raw     string
    wantErr bool   
  }{
    {"public https", "https://example.com", false},
    {"loopback",     "http://127.0.0.1/",   true},
    {"private lan",  "http://192.168.1.1/", true},
    {"bad scheme",   "ftp://example.com",  true},
  }

  for _, tc := range cases{
    t.Run(tc.name, func(t *testing.T){
      err := SafeURL(tc.raw)

      if tc.wantErr{
        if err == nil{
          t.Fatalf("SafeURL(%q) = nil, want error", tc.raw)
			}
		}else{
        if err != nil{
          t.Fatalf("SafeURL(%q) = %v, want nil", tc.raw, err)
			}
	  	}
  	})
  }
}

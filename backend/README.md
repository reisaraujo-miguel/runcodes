To test the server you can run start it with the debug flag:

```bash
go run . -debug
```

A then use the debug authentication to retrieve a valid jwt token:

```bash
curl -v http://localhost:8443/debugAuth
```

**_IMPORTANT: you must change the jwt secret set in .env.example in production to avoid auth bypassing_**

You can use this debug token to make protected API calls. Here is an example of creating a new offering:

```bash
curl -H"Authorization: BEARER $token" -d '{"email": "admin@admin", "name": "test", "end_date": "2027-01-01"}' -v http://localhost:8443/api/offerings/create
```

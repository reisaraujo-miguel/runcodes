To test the server you can run:

```bash
curl -v http://localhost:8443/debugAuth
```

You will get a token you can use to make protected API calls

Here is an example of creating a new offering:

```bash
curl -H"Authorization: BEARER $token" -d '{"email": "admin@admin", "name": "test", "end_date": "2027-01-01"}' -v http://localhost:8443/api/offerings/create
```

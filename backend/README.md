To test the server you can start it with the debug flag:

```bash
go run . -debug
```

To make api calls to protected api's you must provide a valid JWT token:

```bash
curl -H"Authorization: BEARER [token]" \
     -d '{"email": "admin@admin", "name": "test", "end_date": "2027-01-01T23:59:59-03:00"}'\
	   -v http://localhost:8443/api/v1/offerings/create
```

You can get a valid JWT token by using the public login api with a valid user:

```bash
curl -d '{"email": "admin@admin", "password": "[password]"}'\
	   -v http://localhost:8443/api/v1/user/login
```

### Migrated (legacy) users

Users imported from the old database by `database/migration/` keep a legacy
`legacy-sha1$` password hash until their first login, when it is transparently
upgraded to bcrypt. For those logins to work, set the old system's fixed
password salt in the environment:

```bash
export RUNCODES_LEGACY_PASSWORD_SALT="<old global salt>"
```

Users created by the new system are unaffected by this variable.



Based upon [grpcurl](https://github.com/fullstorydev/grpcurl).  

For convenience following examples, please clone `fullstorydev/grpcurl` into `${HOME}/stackql/grpcurl`, eg with `mkdir -p ${HOME}/stackql/grpcurl && git clone https://github.com/fullstorydev/grpcurl`.

## Bankdemo example


Build bankdemo to current dir:

```bash

go build -o bankdemo "${HOME}/stackql/grpcurl/internal/testing/cmd/bankdemo"


```


Basic request response:



```bash
grpcurl -plaintext -H 'Authorization: token joeblow' -d '{ "initial_deposit_cents": 20, "type": 2 }' 127.0.0.1:12345 Bank/OpenAccount
```

```json
{
  "accountNumber": "1",
  "type": "SAVING",
  "balanceCents": 20
}
```

```bash
grpcurl -plaintext -H 'Authorization: token joeblow'  127.0.0.1:12345 Bank/GetAccounts
```

```json
{
  "accounts": [
    {
      "accountNumber": "1",
      "type": "SAVING",
      "balanceCents": 20
    }
  ]
}
```

Results in:

```json

{
  "accountNumber": "1",
  "type": "SAVING",
  "balanceCents": 20
}

```

Then:

```bash



```


Full Duplex streaming:


```bash

grpcurl -plaintext -H 'Authorization: token joeblow' -d '{ "init": {}  }' -import-path ${HOME}/stackql/grpcurl/internal/testing/cmd/bankdemo -proto support.proto  127.0.0.1:12345 Support/ChatCustomer

```

When starting a new session, server writes:

```json
{
  "session": {
    "sessionId": "000002",
    "customerName": "joeblow"
  }
}
```

When rejoining an existing session, server writes:

```json


{
  "session": {
    "sessionId": "000006",
    "customerName": "joeblow",
    "history": [
      {
        "date": "2025-09-17T07:29:37.764312Z",
        "customerMsg": "Hello I am angry!"
      },
      {
        "date": "2025-09-17T07:29:37.764314Z",
        "customerMsg": "Can you please fix my account?"
      },
      {
        "date": "2025-09-17T07:31:13.941966Z",
        "customerMsg": "Hello again I am now somewhat calm."
      }
    ]
  }
}

```

Eg

```bash

grpcurl -plaintext -H 'Authorization: token joeblow' -d '{ "init": { "resume_session_id": "000002" } } { "msg": "Hello I am angry!"  } {"hang_up": 0 } { "init": { "resume_session_id": "000002" } }  ' -import-path ${HOME}/stackql/grpcurl/internal/testing/cmd/bankdemo -proto support.proto  127.0.0.1:12345 Support/ChatCustomer

```

```bash

grpcurl -plaintext -H 'Authorization: token joeblow' -d '{ "init": { } } { "msg": "Hello I am angry!"  } { "msg": "Can you please fix my account?"  } ' -import-path ${HOME}/stackql/grpcurl/internal/testing/cmd/bankdemo -proto support.proto  127.0.0.1:12345 Support/ChatCustomer

```


```bash

grpcurl -plaintext -H 'Authorization: token joeblow' -d '{ "init": { "resume_session_id": "000006"  } } { "msg": "Hello again I am now somewhat calm."  } { "hang_up": 3 } { "init": { } } { "hang_up": 3 } { "init": { } }  { "hang_up": 3 } { "init": { "resume_session_id": "000006"  } } ' -import-path ${HOME}/stackql/grpcurl/internal/testing/cmd/bankdemo -proto support.proto  127.0.0.1:12345 Support/ChatCustomer

```


```bash

grpcurl -plaintext -H 'Authorization: token joeblow' -d '{ "init": { "resume_session_id": "000006"  } } { "msg": "Hello I am angry!"  } { "hang_up": 3 } ' -import-path ${HOME}/stackql/grpcurl/internal/testing/cmd/bankdemo -proto support.proto  127.0.0.1:12345 Support/ChatCustomer

```

```json

```

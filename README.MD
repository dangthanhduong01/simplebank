generate Go code from sql :sqlc generate

if grpc call failse run package pb      and continue
                        show service
                        service Simplebank


UPDATE users
SET 
  hashed_password = CASE 
    WHEN @set_hash_password::boolean = TRUE THEN @hashed_password
    ELSE hashed_password
  END,
  full_name = CASE
    WHEN @set_full_name::boolean = TRUE THEN @full_name
    ELSE full_name
  END,
  email = CASE
    WHEN @set_email::boolean = TRUE THEN @email
    ELSE email
  END
WHERE
  username = @username
RETURNING *;
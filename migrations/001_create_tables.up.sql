CREATE TABLE IF NOT EXISTS users (
  id serial primary key,
  name varchar(100),
  age smallint
);

CREATE TABLE IF NOT EXISTS todos (
  id serial primary key,
  title varchar(300),
  content varchar(500),
  userid integer
);

ALTER TABLE todos ADD CONSTRAINT fk_userid FOREIGN KEY (userid) REFERENCES users(id) ON DELETE CASCADE;

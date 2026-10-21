CREATE ROLE invoicegen_operator NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
GRANT CONNECT ON DATABASE invoicegen_test TO invoicegen_operator;
GRANT USAGE ON SCHEMA public TO invoicegen_operator;

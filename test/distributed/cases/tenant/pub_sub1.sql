create database db1;
create publication pubname1 database db1 account test_tenant_1 comment 'publish db1 database';
create publication pubname2 database db1 account test_tenant_1 comment 'publish db1 database';
set global syspublications = "pubname1,pubname2";
create account test_tenant_2 admin_name 'test_account' identified by '111';
-- @session:id=1&user=test_tenant_2:test_account&password=111
show subscriptions;
show databases;
-- @session
set global syspublications = default;
drop account test_tenant_2;
drop publication pubname1;
drop publication pubname2;
drop database db1;

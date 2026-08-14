-- 归并后的业务引用和已停用重复行不做反向扩散，避免破坏后续交易数据。
DROP INDEX IF EXISTS ux_contact_methods_one_enabled_linuxdo;

INSERT INTO system_configs (
    id,
    config_group,
    config_key,
    name,
    description,
    value_type,
    value,
    default_value,
    is_editable,
    is_system,
    sort_order
)
VALUES
    ('syscfg-platform-name', 'platform', 'site_name', '平台名称', '管理后台展示名称', 'string', 'Grove Console', 'Grove Console', true, true, 10),
    ('syscfg-platform-domain', 'platform', 'site_domain', '平台域名', '平台公开访问域名', 'string', '', '', true, false, 20),
    ('syscfg-storage-max-size', 'storage', 'upload_max_mb', '上传大小上限', '前端演示页展示用上传大小限制（MB）', 'int', '20', '20', true, true, 10),
    ('syscfg-feature-sts', 'feature', 'storage_sts_enabled', '启用 STS 直传', '控制台存储演示页是否展示 STS 凭证区块', 'bool', 'true', 'true', true, true, 10)
ON CONFLICT (id) DO UPDATE
SET
    config_group = EXCLUDED.config_group,
    config_key = EXCLUDED.config_key,
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    value_type = EXCLUDED.value_type,
    value = EXCLUDED.value,
    default_value = EXCLUDED.default_value,
    is_editable = EXCLUDED.is_editable,
    is_system = EXCLUDED.is_system,
    sort_order = EXCLUDED.sort_order,
    updated_at = NOW();

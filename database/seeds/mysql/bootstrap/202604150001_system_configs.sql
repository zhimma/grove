INSERT IGNORE INTO system_configs (
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
    ('syscfg-platform-name', 'platform', 'site_name', '平台名称', '管理后台展示名称', 'string', 'Grove Console', 'Grove Console', TRUE, TRUE, 10),
    ('syscfg-platform-domain', 'platform', 'site_domain', '平台域名', '平台公开访问域名', 'string', '', '', TRUE, FALSE, 20),
    ('syscfg-site-title', 'site', 'title', '站点标题', '前台站点页面标题', 'string', 'Grove', 'Grove', TRUE, FALSE, 10),
    ('syscfg-site-description', 'site', 'description', '站点描述', '前台站点描述和 SEO 摘要', 'string', '', '', TRUE, FALSE, 20),
    ('syscfg-site-logo', 'site', 'logo', '站点 Logo', '前台站点 Logo 地址', 'string', '', '', TRUE, FALSE, 30),
    ('syscfg-site-icp', 'site', 'icp', '备案信息', '站点底部备案信息', 'string', '', '', TRUE, FALSE, 40),
    ('syscfg-storage-max-size', 'storage', 'upload_max_mb', '上传大小上限', '前端演示页展示用上传大小限制（MB）', 'int', '20', '20', TRUE, TRUE, 10),
    ('syscfg-feature-sts', 'feature', 'storage_sts_enabled', '启用 STS 直传', '控制台存储演示页是否展示 STS 凭证区块', 'bool', 'true', 'true', TRUE, TRUE, 10);

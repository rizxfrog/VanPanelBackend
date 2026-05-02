-- 文件分享系统存储过程
-- 数据库: PostgreSQL

-- 1. 生成唯一的分享码 (32位)
CREATE OR REPLACE FUNCTION generate_share_code()
RETURNS VARCHAR(32) AS $$
DECLARE
    v_code VARCHAR(32);
    v_exists BOOLEAN;
BEGIN
    LOOP
        -- 生成32位随机字符串 (字母数字)
        v_code := upper(substr(md5(random()::text || clock_timestamp()::text), 1, 32));
        
        -- 检查是否已存在
        SELECT EXISTS(SELECT 1 FROM cl_file_shares WHERE share_code = v_code) INTO v_exists;
        
        -- 如果不存在则返回
        IF NOT v_exists THEN
            RETURN v_code;
        END IF;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- 2. 生成6位提取码 (字母数字)
CREATE OR REPLACE FUNCTION generate_access_code()
RETURNS VARCHAR(6) AS $$
DECLARE
    v_chars TEXT := 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789';
    v_code VARCHAR(6) := '';
    v_i INTEGER;
BEGIN
    FOR v_i IN 1..6 LOOP
        v_code := v_code || substr(v_chars, floor(random() * length(v_chars) + 1)::integer, 1);
    END LOOP;
    RETURN v_code;
END;
$$ LANGUAGE plpgsql;

-- 3. 创建分享链接
CREATE OR REPLACE FUNCTION create_file_share(
    p_creator_id INTEGER,
    p_access_level VARCHAR(20),
    p_max_downloads INTEGER,
    p_expire_at TIMESTAMP WITH TIME ZONE,
    p_items JSONB  -- [{file_path: "...", file_type: "file", file_name: "..."}]
)
RETURNS TABLE (
    share_id INTEGER,
    share_code VARCHAR(32),
    access_code VARCHAR(6)
) AS $$
DECLARE
    v_share_id INTEGER;
    v_share_code VARCHAR(32);
    v_access_code VARCHAR(6);
    v_item JSONB;
BEGIN
    -- 生成唯一的 share_code 和 access_code
    v_share_code := generate_share_code();
    v_access_code := generate_access_code();
    
    -- 插入分享记录
    INSERT INTO cl_file_shares (
        share_code, access_code, creator_id, access_level, 
        max_downloads, download_count, expire_at, status,
        created_at, updated_at
    ) VALUES (
        v_share_code, v_access_code, p_creator_id, p_access_level,
        p_max_downloads, 0, p_expire_at, 'active',
        NOW(), NOW()
    ) RETURNING id INTO v_share_id;
    
    -- 插入分享项目
    FOR v_item IN SELECT * FROM jsonb_array_elements(p_items)
    LOOP
        INSERT INTO cl_file_share_items (
            share_id, file_path, file_type, file_name,
            created_at, updated_at
        ) VALUES (
            v_share_id,
            v_item->>'file_path',
            v_item->>'file_type',
            v_item->>'file_name',
            NOW(),
            NOW()
        );
    END LOOP;
    
    RETURN QUERY SELECT v_share_id, v_share_code, v_access_code;
END;
$$ LANGUAGE plpgsql;

-- 4. 合并多个分享链接
CREATE OR REPLACE FUNCTION merge_file_shares(
    p_share_ids INTEGER[]
)
RETURNS TABLE (
    share_id INTEGER,
    share_code VARCHAR(32),
    access_code VARCHAR(6)
) AS $$
DECLARE
    v_creator_id INTEGER;
    v_access_level VARCHAR(20);
    v_new_share_id INTEGER;
    v_new_share_code VARCHAR(32);
    v_new_access_code VARCHAR(6);
BEGIN
    -- 验证所有分享都存在且属于同一创建者
    SELECT DISTINCT fs.creator_id INTO v_creator_id
    FROM cl_file_shares fs
    WHERE fs.id = ANY(p_share_ids) AND fs.status = 'active';
    
    IF v_creator_id IS NULL THEN
        RAISE EXCEPTION 'No valid active shares found';
    END IF;
    
    -- 验证所有分享都属于同一创建者
    IF (SELECT COUNT(DISTINCT creator_id) FROM cl_file_shares WHERE id = ANY(p_share_ids) AND status = 'active') > 1 THEN
        RAISE EXCEPTION 'All shares must belong to the same creator';
    END IF;
    
    -- 获取最宽松的访问级别 (如果有任何一个公开，则合并后公开)
    SELECT CASE 
        WHEN COUNT(*) > 0 AND BOOL_OR(access_level = 'public') 
        THEN 'public' 
        ELSE 'login_required' 
    END INTO v_access_level
    FROM cl_file_shares
    WHERE id = ANY(p_share_ids) AND status = 'active';
    
    -- 生成新的分享码和提取码
    v_new_share_code := generate_share_code();
    v_new_access_code := generate_access_code();
    
    -- 创建新的合并分享
    INSERT INTO cl_file_shares (
        share_code, access_code, creator_id, access_level,
        max_downloads, download_count, expire_at, status,
        created_at, updated_at
    ) VALUES (
        v_new_share_code, v_new_access_code, v_creator_id, v_access_level,
        0, 0, NULL, 'active',
        NOW(), NOW()
    ) RETURNING id INTO v_new_share_id;
    
    -- 复制所有项目到新分享
    INSERT INTO cl_file_share_items (share_id, file_path, file_type, file_name, created_at, updated_at)
    SELECT v_new_share_id, fsi.file_path, fsi.file_type, fsi.file_name, NOW(), NOW()
    FROM cl_file_share_items fsi
    WHERE fsi.share_id = ANY(p_share_ids);
    
    -- 标记原分享为已合并
    UPDATE cl_file_shares
    SET status = 'merged', updated_at = NOW()
    WHERE id = ANY(p_share_ids);
    
    RETURN QUERY SELECT v_new_share_id, v_new_share_code, v_new_access_code;
END;
$$ LANGUAGE plpgsql;

-- 5. 验证分享访问权限
CREATE OR REPLACE FUNCTION verify_share_access(
    p_share_code VARCHAR(32),
    p_access_code VARCHAR(6)
)
RETURNS TABLE (
    share_id INTEGER,
    is_valid BOOLEAN,
    error_message VARCHAR(100)
) AS $$
DECLARE
    v_share RECORD;
BEGIN
    -- 查找分享
    SELECT * INTO v_share
    FROM cl_file_shares
    WHERE share_code = p_share_code AND status = 'active';
    
    IF NOT FOUND THEN
        RETURN QUERY SELECT NULL::INTEGER, FALSE, 'Share not found'::VARCHAR(100);
        RETURN;
    END IF;
    
    -- 检查是否过期
    IF v_share.expire_at IS NOT NULL AND v_share.expire_at < NOW() THEN
        UPDATE cl_file_shares SET status = 'expired', updated_at = NOW() WHERE id = v_share.id;
        RETURN QUERY SELECT v_share.id, FALSE, 'Share expired'::VARCHAR(100);
        RETURN;
    END IF;
    
    -- 检查下载次数限制
    IF v_share.max_downloads > 0 AND v_share.download_count >= v_share.max_downloads THEN
        RETURN QUERY SELECT v_share.id, FALSE, 'Download limit reached'::VARCHAR(100);
        RETURN;
    END IF;
    
    -- 验证提取码
    IF v_share.access_code != p_access_code THEN
        RETURN QUERY SELECT v_share.id, FALSE, 'Invalid access code'::VARCHAR(100);
        RETURN;
    END IF;
    
    RETURN QUERY SELECT v_share.id, TRUE, NULL::VARCHAR(100);
END;
$$ LANGUAGE plpgsql;

-- 6. 更新分享设置
CREATE OR REPLACE FUNCTION update_file_share(
    p_share_id INTEGER,
    p_access_level VARCHAR(20) DEFAULT NULL,
    p_max_downloads INTEGER DEFAULT NULL,
    p_expire_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    p_status VARCHAR(20) DEFAULT NULL
)
RETURNS VOID AS $$
BEGIN
    UPDATE cl_file_shares
    SET access_level = COALESCE(p_access_level, access_level),
        max_downloads = COALESCE(p_max_downloads, max_downloads),
        expire_at = p_expire_at,
        status = COALESCE(p_status, status),
        updated_at = NOW()
    WHERE id = p_share_id;
END;
$$ LANGUAGE plpgsql;

-- 7. 获取分享的文件列表
CREATE OR REPLACE FUNCTION get_share_files(
    p_share_code VARCHAR(32)
)
RETURNS TABLE (
    file_path VARCHAR(500),
    file_type VARCHAR(20),
    file_name VARCHAR(255)
) AS $$
BEGIN
    RETURN QUERY
    SELECT fsi.file_path, fsi.file_type, fsi.file_name
    FROM cl_file_share_items fsi
    JOIN cl_file_shares fs ON fs.id = fsi.share_id
    WHERE fs.share_code = p_share_code AND fs.status = 'active';
END;
$$ LANGUAGE plpgsql;

-- 8. 增加下载次数
CREATE OR REPLACE FUNCTION increment_download_count(
    p_share_id INTEGER
)
RETURNS VOID AS $$
BEGIN
    UPDATE cl_file_shares
    SET download_count = download_count + 1,
        updated_at = NOW()
    WHERE id = p_share_id;
END;
$$ LANGUAGE plpgsql;

-- 9. 获取分享统计信息
CREATE OR REPLACE FUNCTION get_share_statistics()
RETURNS TABLE (
    total_shares BIGINT,
    active_shares BIGINT,
    expired_shares BIGINT,
    total_downloads BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        COUNT(*) AS total_shares,
        COUNT(*) FILTER (WHERE status = 'active') AS active_shares,
        COUNT(*) FILTER (WHERE status = 'expired') AS expired_shares,
        COALESCE(SUM(download_count), 0) AS total_downloads
    FROM cl_file_shares;
END;
$$ LANGUAGE plpgsql;

-- 10. 清理过期分享 (定时任务调用)
CREATE OR REPLACE FUNCTION cleanup_expired_shares()
RETURNS INTEGER AS $$
DECLARE
    v_count INTEGER;
BEGIN
    UPDATE cl_file_shares
    SET status = 'expired', updated_at = NOW()
    WHERE status = 'active' 
      AND expire_at IS NOT NULL 
      AND expire_at < NOW();
    
    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count;
END;
$$ LANGUAGE plpgsql;

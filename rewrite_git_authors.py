#!/usr/bin/env python3
import subprocess
import sys
import shlex

def run_cmd(cmd, capture=False, input_data=None):
    """安全执行命令，返回 subprocess.CompletedProcess"""
    result = subprocess.run(
        cmd,
        capture_output=capture,
        text=True,
        input=input_data
    )
    return result

def main():
    # 1. 检查当前目录是否为 Git 仓库
    if run_cmd(['git', 'rev-parse', '--git-dir'], capture=True).returncode != 0:
        print("❌ 错误: 当前目录不是 Git 仓库。", file=sys.stderr)
        sys.exit(1)

    # 2. 获取当前 Git 用户配置
    name = run_cmd(['git', 'config', 'user.name'], capture=True).stdout.strip()
    email = run_cmd(['git', 'config', 'user.email'], capture=True).stdout.strip()

    if not name or not email:
        print("❌ 错误: 未检测到当前 Git 用户信息。请先配置:")
        print("   git config user.name '你的名字'")
        print("   git config user.email '你的邮箱'", file=sys.stderr)
        sys.exit(1)

    print(f"📦 目标: 将仓库中所有提交人信息修改为:")
    print(f"   名称: {name}")
    print(f"   邮箱: {email}")

    # 3. 二次确认
    confirm = input("\n⚠️  此操作会重写整个 Git 历史，原有 Commit Hash 将全部改变。确认继续？(输入 yes): ")
    if confirm.lower() != 'yes':
        print("已取消操作。")
        sys.exit(0)

    # 4. 安全转义（防止姓名/邮箱包含特殊字符导致 Shell 注入）
    safe_name = shlex.quote(name)
    safe_email = shlex.quote(email)

    # 5. 构建 filter-branch 命令
    filter_script = f"""
        export GIT_AUTHOR_NAME={safe_name}
        export GIT_AUTHOR_EMAIL={safe_email}
        export GIT_COMMITTER_NAME={safe_name}
        export GIT_COMMITTER_EMAIL={safe_email}
    """

    cmd = [
        'git', 'filter-branch', '--env-filter', filter_script,
        '--tag-name-filter', 'cat', '--', '--branches', '--tags'
    ]

    print("\n🔄 正在重写提交历史... (可能需要几分钟，请耐心等待)")
    # 直接继承标准输出，便于观察进度
    result = subprocess.run(cmd)
    
    if result.returncode != 0:
        print("\n❌ 历史重写失败，请检查上方错误信息。", file=sys.stderr)
        print("💡 如果提示 'refs/original' 已存在，请先执行清理命令后重试。")
        sys.exit(1)

    # 6. 完成提示
    print("\n✅ 历史重写完成！")
    print("\n📌 后续必要步骤:")
    print("1️⃣ 清理 filter-branch 产生的备份引用 (释放空间):")
    print("   git for-each-ref --format='delete %(refname)' refs/original/ | git update-ref --stdin")
    print("   git for-each-ref --format='%(refname)' refs/original/ | ForEach-Object { git update-ref -d $_ }")
    print("2️⃣ 验证修改结果:")
    print("   git log --format='%an <%ae>' | sort -u")
    print("3️⃣ 强制推送到远程仓库 (⚠️ 谨慎操作，会覆盖远程历史):")
    print("   git push --force --all")
    print("   git push --force --tags")

if __name__ == '__main__':
    main()
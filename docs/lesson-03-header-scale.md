# 第三课 · 顶栏品牌比例调整

用户新批注取代此前50px视觉尺寸：顶栏68px，CampusClaw桌面22px，副标题11px，圆形标记36px；窄屏品牌18px、隐藏副标题并缩短用户名省略宽度。body网格和header共用--header-height，内容区顶部实测68px。只修改web/style.css和index.html静态版本，不改业务JS。

浏览器当前教师页面实测1463、768、390、320px宽，顶栏均68px，无横向溢出或品牌/导航重叠；桌面字体22/11px，手机18px。测试后恢复原视口；15项前端回归通过，原始输出.local/header-ui-tests.txt。此前50px布局文档和作业证据保留为历史，本次为独立小变更adjust-header-brand-scale。无需Docker构建或重启，刷新页面生效。

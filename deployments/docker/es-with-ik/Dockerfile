# ES 8.x + IK 中文分词插件
# 版本必须与 go-elasticsearch/v8 (v8.19.6) 兼容 → 使用 ES 8.12.0
# IK 插件版本必须与 ES 版本严格一致
FROM docker.elastic.co/elasticsearch/elasticsearch:8.12.0

# 安装 IK Analysis 插件（中文分词）
RUN elasticsearch-plugin install --batch \
    https://release.infinilabs.com/analysis-ik/stable/elasticsearch-analysis-ik-8.12.0.zip

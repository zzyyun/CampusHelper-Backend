const BASE_URL = 'https://rithupc.cn/api/v1'
const POST_TYPE = { GENERAL: 1, LOST_FOUND: 2, SECOND_HAND: 3 }
const POST_TYPE_TEXT = { 1: '通用', 2: '失物招领', 3: '二手交易' }
const POST_TYPE_TAG = { 1: 'tag-blue', 2: 'tag-orange', 3: 'tag-green' }
const ITEM_CATEGORY = { DIGITAL: 1, CERTIFICATE: 2, KEY: 3, BOOK: 4, CLOTHING: 5, CHARGER: 6, DAILY: 7, OTHER: 8 }
const ITEM_CATEGORY_TEXT = { 1: '手机/数码', 2: '证件/卡类', 3: '钥匙/门卡', 4: '书籍/资料', 5: '服装/饰品', 6: '充电器/配件', 7: '生活用品', 8: '其他' }
const TASK_TYPE = { DELIVERY: 1, CARPOOL: 2, BOUNTY: 3 }
const TASK_TYPE_TEXT = { 1: '跑腿代拿', 2: '组队/拼车', 3: '悬赏求助' }
const TASK_TYPE_TAG = { 1: 'tag-blue', 2: 'tag-green', 3: 'tag-orange' }
const TASK_STATUS = { OPEN: 1, IN_PROGRESS: 2, COMPLETED: 3, CANCELLED: 4, EXPIRED: 5 }
const TASK_STATUS_TEXT = { 1: '待接单', 2: '进行中', 3: '已完成', 4: '已取消', 5: '已过期' }
const POST_STATUS = { PENDING: 1, PUBLISHED: 2, EXPIRED: 3, CLOSED: 4, REJECTED: 5, RETRIEVED: 6, SOLD: 7 }
const CONDITION_TEXT = { 1: '全新', 2: '几乎全新', 3: '良好', 4: '一般' }
const TRADE_METHOD_TEXT = { 1: '面交', 2: '快递' }
const LOST_OR_FOUND_TEXT = { 1: '我丢失了', 2: '我拾到了' }
const NOTIFY_TYPE = { LIKED: 'liked', PUBLISHED: 'published', REVIEW_RESULT: 'review_result', TAKEN_DOWN: 'taken_down', REPLIED: 'replied' }
const NOTIFY_TYPE_TEXT = { liked: '点赞', published: '发布成功', review_result: '审核结果', taken_down: '违规下架', replied: '回复' }
const SORT_TYPE = { TIME_DESC: 1, LIKES_DESC: 2, RELEVANCE: 3 }
const PAGE_SIZE = 20
module.exports = {
  BASE_URL, POST_TYPE, POST_TYPE_TEXT, POST_TYPE_TAG,
  ITEM_CATEGORY, ITEM_CATEGORY_TEXT,
  TASK_TYPE, TASK_TYPE_TEXT, TASK_TYPE_TAG,
  TASK_STATUS, TASK_STATUS_TEXT, POST_STATUS,
  CONDITION_TEXT, TRADE_METHOD_TEXT, LOST_OR_FOUND_TEXT,
  NOTIFY_TYPE, NOTIFY_TYPE_TEXT, SORT_TYPE, PAGE_SIZE
}

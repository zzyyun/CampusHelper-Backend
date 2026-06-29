const api=require('../../utils/api')
const {POST_TYPE_TEXT,POST_TYPE_TAG,PAGE_SIZE}=require('../../utils/constants')
Page({
  data:{keyword:'',results:[],page:0,total:0,hasMore:false,loading:false,typeFilter:0,postTypeText:POST_TYPE_TEXT,postTypeTag:POST_TYPE_TAG},
  onSearch(){if(!this.data.keyword.trim())return;this.data.page=0;this.doSearch(true)},
  onInput(e){this.setData({keyword:e.detail.value})},
  onTypeFilter(e){const t=parseInt(e.currentTarget.dataset.type);this.setData({typeFilter:t});if(this.data.results.length>0){this.data.page=0;this.doSearch(true)}},
  doSearch(r){if(this.data.loading)return;this.setData({loading:true});const page=r?1:this.data.page+1
    api.searchContent({keyword:this.data.keyword.trim(),type:this.data.typeFilter,page,pageSize:PAGE_SIZE,sort:3}).then(d=>{const list=r?(d.posts||[]):this.data.results.concat(d.posts||[]);this.setData({results:list,page,total:d.total||0,hasMore:list.length<(d.total||0),loading:false})}).catch(()=>this.setData({loading:false}))},
  onLoadMore(){if(this.data.hasMore&&!this.data.loading)this.doSearch()},
  onPostTap(e){wx.navigateTo({url:'/pages/post/detail?id='+e.currentTarget.dataset.id})}
})

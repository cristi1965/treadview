// Background service worker for TradingAgents Extension

chrome.runtime.onInstalled.addListener(() => {
  console.log('我不是神 · AI 交易员伴侣扩展已成功安装！');
});

// Relay messages if needed between content script and popup
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.type === 'PING') {
    sendResponse({ status: 'OK' });
  }
  return true;
});

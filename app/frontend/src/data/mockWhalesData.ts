import type { Investor, CongressMember } from '../types/whales';

// 模拟机构投资者数据
export const mockInvestors: Investor[] = [
  {
    id: '1',
    name: '霍华德·马克斯',
    nameEn: 'Howard Marks',
    slug: 'howard-marks',
    company: 'Oaktree Capital Management',
    holdings: 40,
    topStock: {
      symbol: 'TRMD',
      name: 'Torm Plc',
      percentage: 15.7
    }
  },
  {
    id: '2',
    name: '沃伦·巴菲特',
    nameEn: 'Warren Buffett',
    slug: 'warren-buffett',
    company: 'Berkshire Hathaway',
    holdings: 45,
    topStock: {
      symbol: 'AAPL',
      name: 'Apple Inc.',
      percentage: 41.2
    }
  },
  {
    id: '3',
    name: '查理·芒格',
    nameEn: 'Charlie Munger',
    slug: 'charlie-munger',
    company: 'Daily Journal Corporation',
    holdings: 12,
    topStock: {
      symbol: 'BABA',
      name: 'Alibaba Group',
      percentage: 38.5
    }
  },
  {
    id: '4',
    name: '比尔·阿克曼',
    nameEn: 'Bill Ackman',
    slug: 'bill-ackman',
    company: 'Pershing Square',
    holdings: 8,
    topStock: {
      symbol: 'CMG',
      name: 'Chipotle',
      percentage: 28.3
    }
  },
  {
    id: '5',
    name: '大卫·泰珀',
    nameEn: 'David Tepper',
    slug: 'david-tepper',
    company: 'Appaloosa Management',
    holdings: 35,
    topStock: {
      symbol: 'AMZN',
      name: 'Amazon',
      percentage: 12.8
    }
  }
];

// 模拟国会议员数据
export const mockCongressMembers: CongressMember[] = [
  {
    id: 'M001245',
    name: 'Christian D. Menefee',
    party: 'D',
    state: 'TX',
    district: '18',
    latestTrade: {
      action: 'sell',
      symbol: 'PINS',
      amount: '$15K-$50K',
      date: '6/11'
    }
  },
  {
    id: 'T000490',
    name: 'David J. Taylor',
    party: 'R',
    state: 'OH',
    district: '02',
    isHot: true,
    latestTrade: {
      action: 'buy',
      symbol: 'GOOGL',
      amount: '$1K-$15K',
      date: '6/5'
    }
  },
  {
    id: 'M001236',
    name: 'Tim Moore',
    party: 'R',
    state: 'NC',
    district: '14',
    isHot: true,
    latestTrade: {
      action: 'buy',
      symbol: 'T',
      amount: '$50K-$100K',
      date: '6/4'
    }
  },
  {
    id: 'M001239',
    name: 'John J. McGuire III',
    party: 'R',
    state: 'VA',
    district: '05',
    latestTrade: {
      action: 'sell',
      symbol: 'DELL',
      amount: '$1K-$15K',
      date: '6/4'
    }
  },
  {
    id: 'C001123',
    name: 'Gilbert Ray Cisneros Jr.',
    party: 'D',
    state: 'CA',
    district: '31',
    isHot: true,
    latestTrade: {
      action: 'buy',
      symbol: 'ARQT',
      amount: '$1K-$15K',
      date: '5/29'
    }
  },
  {
    id: 'G000583',
    name: 'Josh Gottheimer',
    party: 'D',
    state: 'NJ',
    district: '05',
    isHot: true,
    latestTrade: {
      action: 'sell',
      symbol: 'ABT',
      amount: '$1K-$15K',
      date: '5/27'
    }
  }
];

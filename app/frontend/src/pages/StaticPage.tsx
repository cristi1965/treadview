import React from 'react';
import { Link } from 'react-router-dom';
import { StockGodShell } from '../components/layout/StockGodShell';
import { useI18n } from '../i18n';

type StaticKind = 'about' | 'terms' | 'privacy' | 'how-to-buy';

interface StaticPageProps {
  kind: StaticKind;
}

const aboutPersonas = [
  { name: '巴菲特', en: 'Buffett', body: '护城河 + 长期 —— 看生意质地、ROE、能力圈,贵了也不追。' },
  { name: '段永平', en: 'Duan', body: '商业模式 + 企业文化 + 不为清单(stop-doing),要“价格合理的好生意”。' },
  { name: '德鲁肯米勒', en: 'Druckenmiller', body: '趋势 + 动量 + 宏观,集中下注、错了快砍。' },
  { name: 'Serenity', en: '@aleabitoreddit', body: '瓶颈狙击 —— 专找供应链上卡脖子、不可替代的节点(半导体 / 光子 / 机器人供应链)。' },
  { name: '情绪', en: 'Sentiment', body: '资金面 + 题材 + 市场情绪,谁在被买、热度在哪。' },
];

const HowToBuyPage: React.FC = () => {
  const { t } = useI18n();
  return (
    <StockGodShell title={t('static.howtoTitle')}>
    <div className="mx-auto max-w-[900px] space-y-5">
      <header className="border-b border-line pb-5">
        <div className="text-[11px] font-medium uppercase tracking-wider text-faint">GUIDE · 2026.06 更新</div>
        <h1 className="mt-2 text-[26px] font-semibold tracking-tight text-ink">如何买美股</h1>
        <p className="mt-2 text-sm leading-relaxed text-muted">
          如果你已经有 crypto,最低门槛的路径不是开传统券商 —— 而是用稳定币(USDC/USDT)在交易所直接买代币化美股。不用 W-8BEN、不用电汇美元、不用等几天审核。
        </p>
      </header>

      <section className="rounded-xl border border-line bg-surface p-5">
        <h2 className="text-sm font-semibold text-ink">为什么不推荐传统券商</h2>
        <div className="mt-4 overflow-x-auto">
          <table className="w-full min-w-[640px] border-collapse text-sm">
            <thead>
              <tr className="border-b border-line text-left text-xs text-muted">
                <th className="py-2 pr-3 font-medium" />
                <th className="py-2 pr-3 font-medium">传统券商<br /><span className="text-faint">盈透 / 嘉信 / 富途</span></th>
                <th className="py-2 font-medium">交易所代币化股票<br /><span className="text-faint">Binance / Bitget</span></th>
              </tr>
            </thead>
            <tbody className="text-muted">
              {[
                ['开户', 'W-8BEN 税表 + 身份审核,几天', '已有交易所账号即可'],
                ['入金', '电汇美元,几天 + 手续费', '稳定币秒到'],
                ['最低', '电汇门槛高(常 $1000+)', '$5（Binance）/ $10（Bitget）'],
                ['交易时间', '美股盘中（你的半夜）', '24/5 或 24/7'],
                ['跨境门槛', '高（税务 + 语言 + 汇率）', '低（crypto 用户已具备）'],
              ].map(([k, a, b]) => (
                <tr key={k} className="border-b border-line/60">
                  <td className="py-2.5 pr-3 text-xs font-medium text-ink">{k}</td>
                  <td className="py-2.5 pr-3 text-xs">{a}</td>
                  <td className="py-2.5 text-xs">{b}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="space-y-4">
        <h2 className="text-sm font-semibold text-ink">两条主流路径</h2>

        <article className="rounded-xl border border-line bg-surface p-5">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <h3 className="text-base font-semibold text-ink">Binance</h3>
            <span className="rounded border border-accent/30 bg-accent/10 px-2 py-0.5 text-[11px] text-accent">2026.06 新上线</span>
          </div>
          <p className="mt-2 text-sm leading-relaxed text-muted">
            7,000+ 美股 + ETF,USDT/USDC/BNB 直接买,碎股 $5 起,零佣金($0.35/单最低费),24/5 交易。代币化层 bStocks(BNB Chain)几周内上线,可自己发起代币化。
          </p>
          <ol className="mt-3 list-decimal space-y-1.5 pl-5 text-sm text-muted">
            <li>登录 Binance App → 找「Stocks / 股票」入口</li>
            <li>用账户里的 USDT / USDC 直接下单</li>
            <li>搜美股代码(如 NVDA / TSLA),$5 起买碎股</li>
            <li>（可选）等 bStocks 上线后,把持仓代币化到链上</li>
          </ol>
          <p className="mt-3 text-xs leading-relaxed text-faint">
            结构:Binance(界面)+ Nest Trading(broker)+ Alpaca(托管/分红)。真股托管,非自托管。
          </p>
          <a
            href="https://www.binance.com"
            target="_blank"
            rel="noreferrer"
            className="mt-4 inline-flex rounded-lg border border-accent/40 bg-accent/10 px-4 py-2 text-sm font-semibold text-accent hover:bg-accent/15"
          >
            注册 Binance →
          </a>
          <p className="mt-2 text-[11px] text-faint">邀请链接 · 含返佣 · 仅非中国大陆地区</p>
        </article>

        <article className="rounded-xl border border-line bg-surface p-5">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <h3 className="text-base font-semibold text-ink">Bitget</h3>
            <span className="rounded border border-line bg-base px-2 py-0.5 text-[11px] text-muted">已上线 · 自托管</span>
          </div>
          <p className="mt-2 text-sm leading-relaxed text-muted">
            走 xStocks 框架(Kraken/Backed 发行),TSLAx / NVDAx / AAPLx / CRCLx / SPYx 等,USDT/USDC/SOL 买,$10 起,24/7,保留私钥自托管。1:1 真股托管背书。
          </p>
          <ol className="mt-3 list-decimal space-y-1.5 pl-5 text-sm text-muted">
            <li>打开 Bitget Wallet（钱包,保留私钥）</li>
            <li>充 USDT / USDC / SOL（从交易所或外部钱包转入）</li>
            <li>进 xStock 板块,选股票(苹果/特斯拉/谷歌…)</li>
            <li>$10 起买碎股,链上秒结算,7×24 可交易</li>
          </ol>
          <p className="mt-3 text-xs leading-relaxed text-faint">
            覆盖 Solana / Base / BNB Chain。自托管 = 你掌握私钥,但也自负保管责任。
          </p>
          <a
            href="https://www.bitget.com"
            target="_blank"
            rel="noreferrer"
            className="mt-4 inline-flex rounded-lg border border-accent/40 bg-accent/10 px-4 py-2 text-sm font-semibold text-accent hover:bg-accent/15"
          >
            注册 Bitget →
          </a>
          <p className="mt-2 text-[11px] text-faint">邀请链接 · 含返佣 · 仅非中国大陆地区</p>
        </article>
      </section>

      <section className="rounded-xl border border-line bg-surface p-5">
        <h2 className="text-sm font-semibold text-ink">这其实就是「链上美元买链上股票」</h2>
        <p className="mt-3 text-sm leading-relaxed text-muted">
          你用 USDC(链上美元)买 CRCLx(Circle 的代币化股票)——稳定币 + RWA 代币化在这里合流。这是 tokenized stocks 赛道在落地:rwa.xyz 数据,代币化股票日交易量已到 $16.8 亿(月 +39%),持有人 29 万+(月 +31%)。赛道在快速长,但仍早期。
        </p>
      </section>

      <section className="rounded-xl border border-line bg-surface p-5">
        <h2 className="text-sm font-semibold text-ink">必读风险</h2>
        <ul className="mt-3 space-y-2 text-sm leading-relaxed text-muted">
          <li>仅限非美用户 —— 这些服务一般不对美国居民开放,看你所在地的可用性</li>
          <li>不是直接股权 —— 代币化股票是「挂钩股价的金融工具」,不等于真持有股份,投票权/某些权利可能没有</li>
          <li>监管不确定 —— SEC 对代币化资产的 innovation exemption 仍在变化,法律框架未定</li>
          <li>对手方风险 —— Binance 走 Nest+Alpaca 分层结构,任一环节出问题可能影响你的持仓</li>
          <li>流动性有限 —— 代币化股票二级市场深度不如真交易所,极端行情可能滑点大</li>
          <li>自托管风险（Bitget）—— 私钥一旦丢失即无法找回</li>
        </ul>
        <p className="mt-4 text-xs leading-relaxed text-faint">
          本页为信息整理,非投资建议,不构成对任何平台的背书。代币化股票涉及监管、对手方、流动性多重风险,交易前请自行研究并确认所在地合规性。数据截至 2026.06,以平台最新公告为准。
        </p>
        <Link to="/whales" className="mt-4 inline-block text-sm font-semibold text-accent hover:underline">
          看名人在买什么 →
        </Link>
      </section>
    </div>
  </StockGodShell>
  );
};

const AboutPage: React.FC = () => {
  const { t } = useI18n();
  return (
  <StockGodShell title={t('static.aboutTitle')}>
    <div className="mx-auto max-w-[900px] space-y-5">
      <header className="border-b border-line pb-5">
        <h1 className="text-[26px] font-semibold tracking-tight text-ink">{t('static.aboutTitle')}</h1>
        <p className="mt-2 text-sm leading-relaxed text-muted">
          {t('static.aboutLead')}
        </p>
      </header>

      <section className="rounded-xl border border-line bg-surface p-5">
        <h2 className="text-sm font-semibold text-ink">{t('static.who')}</h2>
        <p className="mt-3 text-sm leading-relaxed text-muted">
          {t('static.whoBody')}
        </p>
        <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {aboutPersonas.map((p) => (
            <div key={p.name} className="rounded-lg border border-line bg-base px-3 py-3">
              <div className="text-sm font-semibold text-ink">{p.name}</div>
              <div className="text-[11px] text-faint">{p.en}</div>
              <p className="mt-2 text-xs leading-relaxed text-muted">{p.body}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="rounded-xl border border-line bg-surface p-5">
        <h2 className="text-sm font-semibold text-ink">重要:人设是 AI 模拟,不是真人</h2>
        <p className="mt-3 text-sm leading-relaxed text-muted">
          巴菲特 / 段永平 / 德鲁肯米勒 / Serenity 等名字,只用于标识“按其公开方法论运行的 AI 智能体”。所有评分、判读、对决里的虚拟盘业绩与持仓,都是 AI 生成的,并非本人真实观点、业绩或持仓,也未获其授权或背书,不代表其本人。仅供研究参考,非投资建议。
        </p>
      </section>

      <section className="rounded-xl border border-line bg-surface p-5">
        <h2 className="text-sm font-semibold text-ink">分怎么算</h2>
        <p className="mt-3 text-sm leading-relaxed text-muted">
          每一方按自己的框架对公司的业务、财务、产业链位置与时机独立评估,给出 0–100 的分与一句判读理由。列表里的“均分”是五方平均;“分歧”是五方评分的极差,数值越大代表越有争议。这些是方法论的结构化表达,不是预测,更不是买卖信号。
        </p>
      </section>

      <section className="rounded-xl border border-line bg-surface p-5">
        <h2 className="text-sm font-semibold text-ink">数据哪来 · 多久更新</h2>
        <p className="mt-3 text-sm leading-relaxed text-muted">
          价格、市盈率等盘面数据来自第三方公开来源（腾讯行情、Yahoo、Nasdaq、新浪）。页面优先读取当前 API 响应，不可用时会显示已标注数据时间的本地快照；只有管理员可在设置页手动刷新快照。五方判读由 AI 按批次生成，不是实时买卖信号。数据可能延迟或有误，以页面的来源、数据时间和过期标识为准。
        </p>
      </section>

      <section className="rounded-xl border border-line bg-surface p-5">
        <h2 className="text-sm font-semibold text-ink">怎么用</h2>
        <ol className="mt-3 list-decimal space-y-2 pl-5 text-sm leading-relaxed text-muted">
          <li>在热力图或列表里找一只票,颜色/分数一眼看出市场怎么看它。</li>
          <li>点开个股,看五方各自的判读、分歧焦点、产业链定位和盘面数据。</li>
          <li>在列表里按“均分”或“分歧”排序、按某一方筛选,定位你想深挖的票。</li>
        </ol>
        <Link to="/market" className="mt-4 inline-flex rounded-lg border border-accent/40 bg-accent/10 px-4 py-2 text-sm font-semibold text-accent hover:bg-accent/15">
          {t('static.start')}
        </Link>
        <div className="mt-4 flex flex-wrap gap-3 text-xs text-faint">
          <Link to="/terms" className="hover:text-ink">{t('home.terms')}</Link>
          <Link to="/privacy" className="hover:text-ink">{t('home.privacy')}</Link>
        </div>
      </section>
    </div>
  </StockGodShell>
  );
};

const copy: Record<'terms' | 'privacy', { title: string; subtitle: string; sections: Array<{ title: string; body: string[] }> }> = {
  terms: {
    title: '服务条款',
    subtitle: '使用本站即表示你理解它只是研究记录和信息整理。',
    sections: [
      {
        title: '1. 这是什么',
        body: [
          '「我不是神 / Not a Stock God」是一个信息整理与个人研究工具,把公开的行情、基本面与多种投资方法论用可视化方式呈现。',
          '本站所有内容仅供信息参考与学习,不构成投资建议,也不构成对任何证券、基金、加密资产、平台或交易所的要约、招揽或背书。',
        ],
      },
      {
        title: '2. 五方人设是 AI 模拟,不是真人',
        body: [
          '本站出现的巴菲特、段永平、德鲁肯米勒、Serenity 等名字,仅用于标识“依据其公开投资方法论运行的 AI 智能体”。',
          '所有评分、判读、虚拟盘业绩与持仓均由 AI 生成,并非本人的真实观点、发言、业绩或持仓,也未获得任何上述人士的授权或背书。',
        ],
      },
      {
        title: '3. 数据来源与“按现状”提供',
        body: [
          '行情与基本面数据来自第三方公开来源,可能延迟、不完整或有误。13F、龙虎榜、国会交易等披露类数据也可能存在滞后和口径差异。',
          '本站按“现状(as-is)”提供,不对任何数据的准确性、及时性或可用性作出担保。第三方来源可能随时变更或中断。',
        ],
      },
      {
        title: '4. 风险自负',
        body: [
          '股票、ETF、加密货币、代币化资产均涉及重大风险,可能损失全部本金。你基于本站信息所做的任何决策,后果由你自行承担。',
          '投资前请自行研究(DYOR),并在需要时咨询有资质的专业人士。任何页面、榜单、评分、热力图或持仓披露都不是买卖指令。',
        ],
      },
      {
        title: '5. 邀请链接与返佣披露',
        body: [
          '部分页面可能包含跳转到第三方平台的邀请链接,本站可能因此获得返佣。这不代表本站对该平台、产品或服务的背书。',
          '是否使用第三方平台、是否满足当地合规要求、是否承担相关费用和风险,均由你自行判断。',
        ],
      },
      {
        title: '6. 地域',
        body: ['你需自行确认在所在司法管辖区访问与使用本站、以及进行任何相关交易是否合法合规。若当地法律不允许,请停止访问和使用。'],
      },
      {
        title: '7. 知识产权',
        body: ['本站的设计、文案与可视化为本站所有。行情数据版权归各自来源方所有。第三方商标、人名仅用于指代与说明,相关权利归各自所有者。'],
      },
      {
        title: '8. 变更与中断',
        body: ['本站可以随时调整页面、数据源、展示逻辑或访问方式,也可能因维护、故障或第三方服务变化而中断。继续使用即表示你接受更新后的条款。'],
      },
    ],
  },
  privacy: {
    title: '隐私政策',
    subtitle: '尽量少收集,尽量存在你的浏览器里。',
    sections: [
      {
        title: '1. 不需要账号',
        body: ['本站没有注册、没有登录,不收集你的姓名、邮箱、电话等个人身份信息。你可以直接浏览公开页面。'],
      },
      {
        title: '2. 匿名访问分析',
        body: [
          '服务器可能记录基础访问日志,用于排错、安全和性能分析。匿名统计只用于了解页面是否可用、加载是否正常、哪些功能需要改进。',
          '这些统计不用于构建跨站用户画像,也不用于识别你的真实身份。',
        ],
      },
      {
        title: '3. 数据存在你自己的浏览器',
        body: [
          '主题(深/浅色)、语言、观察列表等偏好优先存在你浏览器的 localStorage 里,只留在你本地、不上传到我们的服务器。',
          '清除浏览器数据、换设备、换浏览器或使用隐私模式后,这些本地数据可能消失。',
        ],
      },
      {
        title: '4. 第三方',
        body: [
          '本站托管和行情数据可能依赖第三方公开来源。点击页面上的邀请链接会跳转到第三方平台,你与该平台的交互适用其自己的隐私政策。',
          '不要在本站输入账户密码、交易密钥、身份证件号码或其他敏感个人信息。',
        ],
      },
      {
        title: '5. 不出售数据',
        body: ['我们不出售、不出租你的任何数据 —— 因为本站本就尽量不收集可识别到个人的数据。'],
      },
      {
        title: '6. 联系',
        body: ['对隐私有疑问,可通过站点页脚提供的渠道联系。本页为通俗说明,非法律意见。'],
      },
    ],
  },
};

export const StaticPage: React.FC<StaticPageProps> = ({ kind }) => {
  if (kind === 'how-to-buy') return <HowToBuyPage />;
  if (kind === 'about') return <AboutPage />;

  const page = copy[kind];

  return (
    <StockGodShell title={page.title}>
      <div className="mx-auto max-w-[900px]">
        <header className="mb-8 border-b border-line pb-5">
          <h1 className="text-[26px] font-semibold tracking-tight text-ink">{page.title}</h1>
          <p className="mt-2 text-sm leading-relaxed text-muted">{page.subtitle}</p>
        </header>

        <div className="space-y-5">
          {page.sections.map((section) => (
            <section key={section.title} className="rounded-xl border border-line bg-surface p-5">
              <h2 className="text-sm font-semibold text-ink">{section.title}</h2>
              <div className="mt-3 space-y-2 text-sm leading-relaxed text-muted">
                {section.body.map((paragraph) => (
                  <p key={paragraph}>{paragraph}</p>
                ))}
              </div>
            </section>
          ))}
        </div>

        <footer className="mt-10 border-t border-line pt-5 text-center text-xs text-faint">
          <Link to="/market" className="text-accent hover:underline">回实验行情</Link>
          <span className="mx-2">·</span>
          Not a Stock God · Not Financial Advice
        </footer>
      </div>
    </StockGodShell>
  );
};

export const NotFoundPage: React.FC = () => {
  const { t } = useI18n();
  return (
  <StockGodShell title="404">
    <div className="flex min-h-[520px] items-center justify-center text-center">
      <div>
        <p className="font-mono text-6xl font-semibold tracking-tight text-accent">404</p>
        <h1 className="mt-4 text-xl font-semibold text-ink">{t('static.nf')}</h1>
        <p className="mt-1.5 text-sm leading-relaxed text-muted">{t('static.nfSub')}</p>
        <div className="mt-7 flex flex-wrap items-center justify-center gap-3">
          <Link to="/market" className="rounded-lg border border-accent/30 px-4 py-2 text-sm font-semibold text-accent">{t('stock.backHeat')}</Link>
          <Link to="/scan" className="rounded-lg border border-line bg-surface px-4 py-2 text-sm text-muted transition hover:text-ink">{t('stock.goScan')}</Link>
        </div>
      </div>
    </div>
  </StockGodShell>
  );
};

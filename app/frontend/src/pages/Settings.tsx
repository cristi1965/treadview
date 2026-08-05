import React, { useEffect, useState } from 'react'
import { useAnalysisStore } from '../stores/analysisStore'
import { Save, RefreshCw } from 'lucide-react'

export const Settings: React.FC = () => {
  const { config, fetchConfig, updateConfig } = useAnalysisStore()

  const [provider, setProvider] = useState('google')
  const [deepModel, setDeepModel] = useState('gemini-3.5-flash')
  const [quickModel, setQuickModel] = useState('gemini-3.1-flash-lite')
  const [lang, setLang] = useState('English')
  const [debateRounds, setDebateRounds] = useState(1)
  const [riskRounds, setRiskRounds] = useState(1)
  const [backendUrl, setBackendUrl] = useState('')
  const [apiKey, setApiKey] = useState('')

  // Provider-specific hints
  const providerKeyHint = (p: string) => {
    switch(p) {
      case 'google': return 'Google API Key (GOOGLE_API_KEY)'
      case 'deepseek': return 'DeepSeek API Key (DEEPSEEK_API_KEY)'
      case 'openai': return 'OpenAI API Key (OPENAI_API_KEY)'
      case 'openai_compatible': return 'API Key (optional for local servers)'
      default: return 'API Key'
    }
  }

  const providerModelHint = (p: string) => {
    switch(p) {
      case 'google': return 'e.g. gemini-3.5-flash'
      case 'deepseek': return 'e.g. deepseek-chat, deepseek-reasoner'
      case 'openai': return 'e.g. gpt-4o, o3-mini'
      case 'openai_compatible': return 'Model ID as served by your endpoint'
      default: return ''
    }
  }

  useEffect(() => {
    fetchConfig()
  }, [])

  useEffect(() => {
    if (config) {
      setProvider(config.llm_provider)
      setDeepModel(config.deep_think_llm)
      setQuickModel(config.quick_think_llm)
      setLang(config.output_language)
      setDebateRounds(config.max_debate_rounds)
      setRiskRounds(config.max_risk_rounds)
      setBackendUrl((config as any).llm_backend_url || '')
    }
  }, [config])

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    const updates: Record<string, any> = {
      llm_provider: provider,
      deep_think_llm: deepModel,
      quick_think_llm: quickModel,
      output_language: lang,
      max_debate_rounds: debateRounds,
      max_risk_rounds: riskRounds,
    }
    if (backendUrl) {
      updates.llm_backend_url = backendUrl
    }
    if (apiKey) {
      if (provider === 'deepseek') {
        updates.deepseek_api_key = apiKey
      } else if (provider === 'google') {
        updates.google_api_key = apiKey
      } else if (provider === 'openai' || provider === 'openai_compatible') {
        updates.openai_api_key = apiKey
      }
    }
    await updateConfig(updates)
    alert('Settings saved successfully! Restart the backend for provider changes to take full effect.')
  }

  return (
    <div style={{ maxWidth: '800px', margin: '0 auto', display: 'flex', flexDirection: 'column', gap: '2rem' }}>
      <div className="card" style={{ padding: '2rem' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '2rem', borderBottom: '1px solid var(--border-color)', paddingBottom: '1rem' }}>
          <div>
            <h2 style={{ fontSize: '1.5rem', color: 'white' }}>System Configuration</h2>
            <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>Configure LLM engine, debates, and API credentials.</p>
          </div>
          <button className="btn btn-secondary" onClick={() => fetchConfig()}>
            <RefreshCw size={16} /> Reload
          </button>
        </div>

        <form onSubmit={handleSave} style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1.5rem' }}>
            <div className="input-group">
              <span className="input-label">LLM Provider</span>
              <select className="text-input" value={provider} onChange={(e) => setProvider(e.target.value)}>
                <option value="google">Google Gemini</option>
                <option value="deepseek">DeepSeek</option>
                <option value="openai">OpenAI</option>
                <option value="openai_compatible">OpenAI-Compatible</option>
              </select>
            </div>

            <div className="input-group">
              <span className="input-label">Output Language</span>
              <select className="text-input" value={lang} onChange={(e) => setLang(e.target.value)}>
                <option value="English">English</option>
                <option value="Chinese">Chinese (简体中文)</option>
              </select>
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1.5rem' }}>
            <div className="input-group">
              <span className="input-label">Deep Thinking Model</span>
              <input type="text" className="text-input" value={deepModel} onChange={(e) => setDeepModel(e.target.value)} placeholder={providerModelHint(provider)} />
              <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{providerModelHint(provider)}</span>
            </div>

            <div className="input-group">
              <span className="input-label">Quick Thinking Model</span>
              <input type="text" className="text-input" value={quickModel} onChange={(e) => setQuickModel(e.target.value)} placeholder={providerModelHint(provider)} />
              <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{providerModelHint(provider)}</span>
            </div>
          </div>

          {provider === 'deepseek' || provider === 'openai_compatible' ? (
            <div className="input-group">
              <span className="input-label">API Base URL</span>
              <input type="text" className="text-input" value={backendUrl} onChange={(e) => setBackendUrl(e.target.value)} placeholder={
                provider === 'deepseek' ? 'https://api.deepseek.com/v1' : 'http://localhost:8000/v1'
              } />
              <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                {provider === 'deepseek' ? 'Default: https://api.deepseek.com/v1' : 'Your vLLM / LM Studio / Ollama endpoint'}
              </span>
            </div>
          ) : null}

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1.5rem' }}>
            <div className="input-group">
              <span className="input-label">Debate Rounds (Research Team)</span>
              <input type="number" min="1" max="5" className="text-input" value={debateRounds} onChange={(e) => setDebateRounds(parseInt(e.target.value) || 1)} />
            </div>

            <div className="input-group">
              <span className="input-label">Risk Discussion Rounds</span>
              <input type="number" min="1" max="5" className="text-input" value={riskRounds} onChange={(e) => setRiskRounds(parseInt(e.target.value) || 1)} />
            </div>
          </div>

          <div className="input-group">
            <span className="input-label">{providerKeyHint(provider)} (Optional Override)</span>
            <input 
              type="password" 
              className="text-input" 
              placeholder="••••••••••••••••••••••••••••••••"
              value={apiKey} 
              onChange={(e) => setApiKey(e.target.value)} 
            />
            <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
              Leave blank to keep key loaded from .env file.
            </span>
          </div>

          <button type="submit" className="btn btn-primary" style={{ marginTop: '1rem', alignSelf: 'flex-start' }}>
            <Save size={18} /> Save Settings
          </button>
        </form>
      </div>
    </div>
  )
}

import { ConnectError } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';

import { api } from '../lib';

/** A plan made by hand: it opens at once, with signups from today. */
export function NewPlan({ cityId, onCreated }: { cityId: string; onCreated: () => void }) {
  const queryClient = useQueryClient();
  const catalog = useQuery({ queryKey: ['catalog-cities'], queryFn: () => api.catalog.listCities({ locale: 'en' }) });
  const activities = useQuery({ queryKey: ['catalog-activities', cityId], queryFn: () => api.catalog.listActivities({ cityId, locale: 'en' }) });
  const city = catalog.data?.cities.find((c) => c.id === cityId);
  const [areaId, setAreaId] = useState('');
  const [activityId, setActivityId] = useState('');
  const [followUpId, setFollowUpId] = useState('');
  const [venue, setVenue] = useState('');
  const [primary, setPrimary] = useState('');
  const [secondary, setSecondary] = useState('');
  const [title, setTitle] = useState('');
  const [summary, setSummary] = useState('');
  const venues = useQuery({
    queryKey: ['venue-options', areaId, activityId],
    queryFn: () => api.review.listVenueOptions({ areaId, activityId }),
    enabled: areaId !== '' && activityId !== '',
  });
  const languages = city?.languages ?? [];
  const create = useMutation({
    mutationFn: () => api.review.createPlan({
      cityId, areaId, activityId, followUpId, venuePlaceId: venue,
      primaryLanguage: primary || languages[0] || 'en', secondaryLanguage: secondary, title, summary,
    }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['review-plans'] });
      void queryClient.invalidateQueries({ queryKey: ['review-cities'] });
      onCreated();
    },
  });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    create.mutate();
  };

  return (
    <form className="card new-plan" onSubmit={submit}>
      <h2>New plan in {city?.name ?? cityId}</h2>
      <div className="grid">
        <label>Area
          <select required value={areaId} onChange={(e) => { setAreaId(e.target.value); setVenue(''); }}>
            <option value="">Choose…</option>
            {city?.areas.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
          </select>
        </label>
        <label>Activity
          <select required value={activityId} onChange={(e) => { setActivityId(e.target.value); setVenue(''); }}>
            <option value="">Choose…</option>
            {activities.data?.activities.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
          </select>
        </label>
        <label>Then (optional)
          <select value={followUpId} onChange={(e) => setFollowUpId(e.target.value)}>
            <option value="">Nothing after</option>
            {activities.data?.activities.filter((a) => a.id !== activityId).map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
          </select>
        </label>
        <label>Venue
          <select value={venue} onChange={(e) => setVenue(e.target.value)} disabled={!venues.data?.venues.length}>
            <option value="">{venues.data && !venues.data.venues.length ? 'None needed or none found: meeting point later' : 'Pick one for me'}</option>
            {/* Approved venues first, then by name (the API returns them by id). */}
            {[...(venues.data?.venues ?? [])].sort((a, b) => Number(b.status === 'approved') - Number(a.status === 'approved') || (a.name || a.placeId).localeCompare(b.name || b.placeId)).map((v) => (
              <option key={v.placeId} value={v.placeId}>{v.name || v.placeId}{v.status === 'approved' ? ' ✓' : ''} ({v.categoryId})</option>
            ))}
          </select>
        </label>
        <label>Language
          <select value={primary || languages[0] || ''} onChange={(e) => setPrimary(e.target.value)}>
            {[...new Set([...languages, 'en'])].map((l) => <option key={l} value={l}>{l}</option>)}
          </select>
        </label>
        <label>Second language (exchange)
          <select value={secondary} onChange={(e) => setSecondary(e.target.value)}>
            <option value="">None</option>
            {[...new Set([...languages, 'en'])].filter((l) => l !== (primary || languages[0])).map((l) => <option key={l} value={l}>{l}</option>)}
          </select>
        </label>
      </div>
      <label className="note">Title <span className="muted">(English, optional; else made from the activities)</span>
        <input maxLength={80} value={title} onChange={(e) => setTitle(e.target.value)} />
      </label>
      <label className="note">Summary <span className="muted">(English, optional)</span>
        <textarea rows={2} maxLength={500} value={summary} onChange={(e) => setSummary(e.target.value)} />
      </label>
      <div className="row">
        <button disabled={create.isPending}>Open the plan</button>
        <span className="muted">It opens straight away, with signups from today.</span>
      </div>
      {(create.error || venues.error) && <p className="error">{ConnectError.from(create.error ?? venues.error).message}</p>}
    </form>
  );
}

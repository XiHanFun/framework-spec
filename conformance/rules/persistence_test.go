package rules_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestIndexNamesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.IndexNames()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxLength != rules.MaxIndexNameLength || vectors.Prefix != rules.IndexNamePrefix || vectors.UniquePrefix != rules.UniqueIndexNamePrefix {
		t.Fatalf("maxLength and prefixes = %d, %q, %q; want %d, %q, %q", vectors.MaxLength, vectors.Prefix, vectors.UniquePrefix,
			rules.MaxIndexNameLength, rules.IndexNamePrefix, rules.UniqueIndexNamePrefix)
	}
	for _, c := range vectors.Abbreviations {
		if got := rules.IndexColumnAbbreviation(c.Property); got != c.Abbreviation {
			t.Errorf("abbreviation of %q = %q, want %q", c.Property, got, c.Abbreviation)
		}
	}
	atLimit, overLimit := false, false
	for _, c := range vectors.Names {
		name, ok := rules.IndexName(c.Table, c.Properties, c.Unique)
		if ok != (c.Index != nil) {
			t.Errorf("%s: %q within the limit = %v, want %v", c.Name, name, ok, c.Index != nil)
			continue
		}
		if c.Index != nil && name != *c.Index {
			t.Errorf("%s: index name = %q, want %q", c.Name, name, *c.Index)
		}
		atLimit = atLimit || (ok && len(name) == vectors.MaxLength)
		overLimit = overLimit || (!ok && len(name) == vectors.MaxLength+1)
	}
	// 上限两侧各要有一条：正好到上限的名字与只多一个字符的名字
	if !atLimit || !overLimit {
		t.Errorf("the vector must keep a derived name of exactly %d characters (found: %v) and one of %d (found: %v)", vectors.MaxLength, atLimit, vectors.MaxLength+1, overLimit)
	}
	replayGrammar(t, "explicit index name", vectors.MaxLength, vectors.ExplicitNames.Valid, vectors.ExplicitNames.Invalid, rules.IsExplicitIndexName)
}

func TestSnowflakeIDsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.SnowflakeIDs()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.TimestampBits != rules.SnowflakeTimestampBits || vectors.WorkerBits != rules.SnowflakeWorkerBits || vectors.SequenceBits != rules.SnowflakeSequenceBits ||
		vectors.MaxWorker != rules.MaxSnowflakeWorker || vectors.MaxSequence != rules.MaxSnowflakeSequence || vectors.WorkerLockPrefix != rules.SnowflakeWorkerLockPrefix {
		t.Fatalf("the bit layout in the vector differs from the rules: %+v", vectors)
	}
	// 三段加符号位正好 64 位
	if vectors.TimestampBits+vectors.WorkerBits+vectors.SequenceBits != 63 {
		t.Errorf("the three parts take %d bits, want 63", vectors.TimestampBits+vectors.WorkerBits+vectors.SequenceBits)
	}
	if epoch, err := time.Parse(time.RFC3339, vectors.DefaultEpoch); err != nil || !epoch.Equal(rules.DefaultSnowflakeEpoch) {
		t.Errorf("default epoch = %s, want %s (%v)", vectors.DefaultEpoch, rules.DefaultSnowflakeEpoch.Format(time.RFC3339), err)
	}
	if got := rules.SnowflakeWorkerLockResource(7); got != vectors.WorkerLockPrefix+"7" {
		t.Errorf("worker lock resource = %q, want %q", got, vectors.WorkerLockPrefix+"7")
	}

	beyondDouble := false
	for _, c := range vectors.Cases {
		value, errValue := strconv.ParseInt(c.Value, 10, 64)
		milliseconds, errMilliseconds := strconv.ParseInt(c.Milliseconds, 10, 64)
		epoch, errEpoch := time.Parse(time.RFC3339, c.Epoch)
		want, errTime := time.Parse(time.RFC3339Nano, c.Time)
		if errValue != nil || errMilliseconds != nil || errEpoch != nil || errTime != nil {
			t.Errorf("%s: the case does not parse: %v %v %v %v", c.Name, errValue, errMilliseconds, errEpoch, errTime)
			continue
		}
		// 两个方向都要对：由三段合成，由标识还原
		if composed, ok := rules.ComposeSnowflakeID(milliseconds, c.Worker, c.Sequence); !ok || composed != value {
			t.Errorf("%s: composed %d (ok %v), want %d", c.Name, composed, ok, value)
		}
		if gotMilliseconds, worker, sequence := rules.ParseSnowflakeID(value); gotMilliseconds != milliseconds || worker != c.Worker || sequence != c.Sequence {
			t.Errorf("%s: parsed %d, %d, %d; want %d, %d, %d", c.Name, gotMilliseconds, worker, sequence, milliseconds, c.Worker, c.Sequence)
		}
		if got := rules.SnowflakeTime(value, epoch); !got.Equal(want) {
			t.Errorf("%s: time = %s, want %s", c.Name, got.Format(time.RFC3339Nano), c.Time)
		}
		beyondDouble = beyondDouble || value > 1<<53
	}
	if !beyondDouble {
		t.Error("the vector must keep an identifier above 2^53")
	}
	for _, c := range vectors.OutOfRange {
		milliseconds, err := strconv.ParseInt(c.Milliseconds, 10, 64)
		if err != nil {
			t.Errorf("%s: %v", c.Name, err)
			continue
		}
		if value, ok := rules.ComposeSnowflakeID(milliseconds, c.Worker, c.Sequence); ok {
			t.Errorf("%s: the parts must be rejected, composed %d", c.Name, value)
		}
	}
}

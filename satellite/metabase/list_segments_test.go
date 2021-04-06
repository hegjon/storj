// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
)

func TestListSegments(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()
		now := time.Now()

		t.Run("StreamID missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListSegments{
				Opts:     metabase.ListSegments{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Invalid limit", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Invalid limit: -1",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("no segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    1,
				},
				Result: metabase.ListSegmentsResult{},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			expectedObject := createObject(ctx, t, db, obj, 10)

			expectedSegment := metabase.Segment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt:         &now,
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1024,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			expectedRawSegments := make([]metabase.RawSegment, 10)
			expectedSegments := make([]metabase.Segment, 10)
			for i := range expectedSegments {
				expectedSegment.Position.Index = uint32(i)
				expectedSegments[i] = expectedSegment
				expectedRawSegments[i] = metabase.RawSegment(expectedSegment)
				expectedSegment.PlainOffset += int64(expectedSegment.PlainSize)
			}

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    10,
				},
				Result: metabase.ListSegmentsResult{
					Segments: expectedSegments,
				},
			}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    1,
				},
				Result: metabase.ListSegmentsResult{
					Segments: expectedSegments[:1],
					More:     true,
				},
			}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    2,
					Cursor: metabase.SegmentPosition{
						Index: 1,
					},
				},
				Result: metabase.ListSegmentsResult{
					Segments: expectedSegments[2:4],
					More:     true,
				},
			}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    2,
					Cursor: metabase.SegmentPosition{
						Index: 10,
					},
				},
				Result: metabase.ListSegmentsResult{
					More: false,
				},
			}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    2,
					Cursor: metabase.SegmentPosition{
						Part:  1,
						Index: 10,
					},
				},
				Result: metabase.ListSegmentsResult{
					More: false,
				},
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(expectedObject),
				},
				Segments: expectedRawSegments,
			}.Check(ctx, t, db)
		})

		t.Run("unordered parts", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			var testCases = []struct {
				segments []metabase.SegmentPosition
			}{
				{[]metabase.SegmentPosition{
					{Part: 3, Index: 0},
					{Part: 0, Index: 0},
					{Part: 1, Index: 0},
					{Part: 2, Index: 0},
				}},
				{[]metabase.SegmentPosition{
					{Part: 3, Index: 0},
					{Part: 2, Index: 0},
					{Part: 1, Index: 0},
					{Part: 0, Index: 0},
				}},
				{[]metabase.SegmentPosition{
					{Part: 0, Index: 0},
					{Part: 2, Index: 0},
					{Part: 1, Index: 0},
					{Part: 3, Index: 0},
				}},
			}

			expectedSegment := metabase.Segment{
				StreamID:          obj.StreamID,
				CreatedAt:         &now,
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1024,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			for _, tc := range testCases {
				obj := randObjectStream()

				BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   defaultTestEncryption,
					},
					Version: obj.Version,
				}.Check(ctx, t, db)

				for i, segmentPosition := range tc.segments {
					BeginSegment{
						Opts: metabase.BeginSegment{
							ObjectStream: obj,
							Position:     segmentPosition,
							RootPieceID:  storj.PieceID{byte(i + 1)},
							Pieces: []metabase.Piece{{
								Number:      1,
								StorageNode: testrand.NodeID(),
							}},
						},
					}.Check(ctx, t, db)

					CommitSegment{
						Opts: metabase.CommitSegment{
							ObjectStream: obj,
							Position:     segmentPosition,
							RootPieceID:  storj.PieceID{1},
							Pieces:       metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},

							EncryptedKey:      []byte{3},
							EncryptedKeyNonce: []byte{4},
							EncryptedETag:     []byte{5},

							EncryptedSize: 1024,
							PlainSize:     512,
							PlainOffset:   0,
							Redundancy:    defaultTestRedundancy,
						},
					}.Check(ctx, t, db)
				}

				CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
					},
				}.Check(ctx, t, db)

				expectedSegments := make([]metabase.Segment, 4)
				expectedOffset := int64(0)
				for i := range expectedSegments {
					expectedSegments[i] = expectedSegment
					expectedSegments[i].StreamID = obj.StreamID
					expectedSegments[i].Position.Part = uint32(i)
					expectedSegments[i].PlainOffset = expectedOffset
					expectedOffset += int64(expectedSegment.PlainSize)
				}

				ListSegments{
					Opts: metabase.ListSegments{
						StreamID: obj.StreamID,
						Limit:    0,
					},
					Result: metabase.ListSegmentsResult{
						Segments: expectedSegments,
					},
				}.Check(ctx, t, db)
			}
		})
	})
}

func TestListStreamPositions(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()
		now := time.Now()

		t.Run("StreamID missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListStreamPositions{
				Opts:     metabase.ListStreamPositions{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Invalid limit", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListStreamPositions{
				Opts: metabase.ListStreamPositions{
					StreamID: obj.StreamID,
					Limit:    -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Invalid limit: -1",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("no segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListStreamPositions{
				Opts: metabase.ListStreamPositions{
					StreamID: obj.StreamID,
					Limit:    1,
				},
				Result: metabase.ListStreamPositionsResult{},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			expectedObject := createObject(ctx, t, db, obj, 10)

			expectedSegment := metabase.Segment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt:         &now,
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1024,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			expectedRawSegments := make([]metabase.RawSegment, 10)
			expectedSegments := make([]metabase.SegmentPositionInfo, 10)
			for i := range expectedSegments {
				expectedSegment.Position.Index = uint32(i)
				expectedRawSegments[i] = metabase.RawSegment(expectedSegment)
				expectedSegments[i] = metabase.SegmentPositionInfo{
					Position:          expectedSegment.Position,
					PlainSize:         expectedSegment.PlainSize,
					PlainOffset:       expectedSegment.PlainOffset,
					CreatedAt:         &now,
					EncryptedKey:      expectedSegment.EncryptedKey,
					EncryptedKeyNonce: expectedSegment.EncryptedKeyNonce,
					EncryptedETag:     expectedSegment.EncryptedETag,
				}
				expectedSegment.PlainOffset += int64(expectedSegment.PlainSize)
			}

			ListStreamPositions{
				Opts: metabase.ListStreamPositions{
					StreamID: obj.StreamID,
					Limit:    10,
				},
				Result: metabase.ListStreamPositionsResult{
					Segments: expectedSegments,
				},
			}.Check(ctx, t, db)

			ListStreamPositions{
				Opts: metabase.ListStreamPositions{
					StreamID: obj.StreamID,
					Limit:    1,
				},
				Result: metabase.ListStreamPositionsResult{
					Segments: expectedSegments[:1],
					More:     true,
				},
			}.Check(ctx, t, db)

			ListStreamPositions{
				Opts: metabase.ListStreamPositions{
					StreamID: obj.StreamID,
					Limit:    2,
					Cursor: metabase.SegmentPosition{
						Index: 1,
					},
				},
				Result: metabase.ListStreamPositionsResult{
					Segments: expectedSegments[2:4],
					More:     true,
				},
			}.Check(ctx, t, db)

			ListStreamPositions{
				Opts: metabase.ListStreamPositions{
					StreamID: obj.StreamID,
					Limit:    2,
					Cursor: metabase.SegmentPosition{
						Index: 10,
					},
				},
				Result: metabase.ListStreamPositionsResult{
					More: false,
				},
			}.Check(ctx, t, db)

			ListStreamPositions{
				Opts: metabase.ListStreamPositions{
					StreamID: obj.StreamID,
					Limit:    2,
					Cursor: metabase.SegmentPosition{
						Part:  1,
						Index: 10,
					},
				},
				Result: metabase.ListStreamPositionsResult{
					More: false,
				},
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(expectedObject),
				},
				Segments: expectedRawSegments,
			}.Check(ctx, t, db)
		})

		t.Run("unordered parts", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			var testCases = []struct {
				segments []metabase.SegmentPosition
			}{
				{[]metabase.SegmentPosition{
					{Part: 3, Index: 0},
					{Part: 0, Index: 0},
					{Part: 1, Index: 0},
					{Part: 2, Index: 0},
				}},
				{[]metabase.SegmentPosition{
					{Part: 3, Index: 0},
					{Part: 2, Index: 0},
					{Part: 1, Index: 0},
					{Part: 0, Index: 0},
				}},
				{[]metabase.SegmentPosition{
					{Part: 0, Index: 0},
					{Part: 2, Index: 0},
					{Part: 1, Index: 0},
					{Part: 3, Index: 0},
				}},
			}

			expectedSegment := metabase.Segment{
				StreamID:          obj.StreamID,
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1024,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			for _, tc := range testCases {
				obj := randObjectStream()

				BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   defaultTestEncryption,
					},
					Version: obj.Version,
				}.Check(ctx, t, db)

				for i, segmentPosition := range tc.segments {
					BeginSegment{
						Opts: metabase.BeginSegment{
							ObjectStream: obj,
							Position:     segmentPosition,
							RootPieceID:  storj.PieceID{byte(i + 1)},
							Pieces: []metabase.Piece{{
								Number:      1,
								StorageNode: testrand.NodeID(),
							}},
						},
					}.Check(ctx, t, db)

					CommitSegment{
						Opts: metabase.CommitSegment{
							ObjectStream: obj,
							Position:     segmentPosition,
							RootPieceID:  storj.PieceID{1},
							Pieces:       metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},

							EncryptedKey:      []byte{3},
							EncryptedKeyNonce: []byte{4},
							EncryptedETag:     []byte{5},

							EncryptedSize: 1024,
							PlainSize:     512,
							PlainOffset:   0,
							Redundancy:    defaultTestRedundancy,
						},
					}.Check(ctx, t, db)
				}

				CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
					},
				}.Check(ctx, t, db)

				expectedSegments := make([]metabase.SegmentPositionInfo, 4)
				expectedOffset := int64(0)
				for i := range expectedSegments {
					pos := expectedSegment.Position
					pos.Part = uint32(i)
					expectedSegments[i] = metabase.SegmentPositionInfo{
						Position:          pos,
						PlainSize:         expectedSegment.PlainSize,
						PlainOffset:       expectedOffset,
						CreatedAt:         &now,
						EncryptedKey:      expectedSegment.EncryptedKey,
						EncryptedKeyNonce: expectedSegment.EncryptedKeyNonce,
						EncryptedETag:     expectedSegment.EncryptedETag,
					}
					expectedOffset += int64(expectedSegment.PlainSize)
				}

				ListStreamPositions{
					Opts: metabase.ListStreamPositions{
						StreamID: obj.StreamID,
						Limit:    0,
					},
					Result: metabase.ListStreamPositionsResult{
						Segments: expectedSegments,
					},
				}.Check(ctx, t, db)
			}
		})

		t.Run("range", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			const segmentCount = 10
			const segmentSize = 512

			expectedSegment := metabase.Segment{
				StreamID:          obj.StreamID,
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1024,
				PlainSize:         segmentSize,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			obj := randObjectStream()

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			for i := 0; i < segmentCount; i++ {
				segmentPosition := metabase.SegmentPosition{
					Part:  uint32(i / 2),
					Index: uint32(i % 2),
				}

				BeginSegment{
					Opts: metabase.BeginSegment{
						ObjectStream: obj,
						Position:     segmentPosition,
						RootPieceID:  storj.PieceID{byte(i + 1)},
						Pieces: []metabase.Piece{{
							Number:      1,
							StorageNode: testrand.NodeID(),
						}},
					},
				}.Check(ctx, t, db)

				CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Position:     segmentPosition,
						RootPieceID:  storj.PieceID{1},
						Pieces:       metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},

						EncryptedKey:      []byte{3},
						EncryptedKeyNonce: []byte{4},
						EncryptedETag:     []byte{5},

						EncryptedSize: 1024,
						PlainSize:     segmentSize,
						PlainOffset:   0,
						Redundancy:    defaultTestRedundancy,
					},
				}.Check(ctx, t, db)
			}

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			expectedSegments := make([]metabase.SegmentPositionInfo, segmentCount)
			expectedOffset := int64(0)
			for i := range expectedSegments {
				segmentPosition := metabase.SegmentPosition{
					Part:  uint32(i / 2),
					Index: uint32(i % 2),
				}
				expectedSegments[i] = metabase.SegmentPositionInfo{
					Position:          segmentPosition,
					PlainSize:         expectedSegment.PlainSize,
					PlainOffset:       expectedOffset,
					CreatedAt:         &now,
					EncryptedKey:      expectedSegment.EncryptedKey,
					EncryptedKeyNonce: expectedSegment.EncryptedKeyNonce,
					EncryptedETag:     expectedSegment.EncryptedETag,
				}
				expectedOffset += int64(expectedSegment.PlainSize)
			}

			ListStreamPositions{
				Opts: metabase.ListStreamPositions{
					StreamID: obj.StreamID,
					Range: &metabase.StreamRange{
						PlainStart: 5,
						PlainLimit: 4,
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "invalid range: 5:4",
			}.Check(ctx, t, db)

			type rangeTest struct {
				limit      int
				plainStart int64
				plainLimit int64
				results    []metabase.SegmentPositionInfo
				more       bool
			}

			totalSize := int64(segmentCount * 512)

			var tests = []rangeTest{
				{plainStart: 0, plainLimit: 0},
				{plainStart: totalSize, plainLimit: totalSize},
				{plainStart: 0, plainLimit: totalSize, results: expectedSegments},
				{plainStart: 0, plainLimit: totalSize - (segmentSize - 1), results: expectedSegments},
				{plainStart: 0, plainLimit: totalSize - segmentSize, results: expectedSegments[:segmentCount-1]},
				{plainStart: 0, plainLimit: segmentSize, results: expectedSegments[:1]},
				{plainStart: 0, plainLimit: segmentSize + 1, results: expectedSegments[:2]},
				{plainStart: segmentSize, plainLimit: totalSize, results: expectedSegments[1:]},
				{plainStart: segmentSize / 2, plainLimit: segmentSize + segmentSize/2, results: expectedSegments[0:2]},
				{plainStart: segmentSize - 1, plainLimit: segmentSize + segmentSize/2, results: expectedSegments[0:2]},
				{plainStart: segmentSize, plainLimit: segmentSize + segmentSize/2, results: expectedSegments[1:2]},
				{plainStart: segmentSize + 1, plainLimit: segmentSize + segmentSize/2, results: expectedSegments[1:2]},
				{limit: 2, plainStart: segmentSize, plainLimit: totalSize, results: expectedSegments[1:3], more: true},
			}
			for _, test := range tests {
				ListStreamPositions{
					Opts: metabase.ListStreamPositions{
						StreamID: obj.StreamID,
						Limit:    test.limit,
						Range: &metabase.StreamRange{
							PlainStart: test.plainStart,
							PlainLimit: test.plainLimit,
						},
					},
					Result: metabase.ListStreamPositionsResult{
						Segments: test.results,
						More:     test.more,
					},
				}.Check(ctx, t, db)
			}
		})
	})
}
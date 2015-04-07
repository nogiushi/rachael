package main

import (
	"bytes"
	"encoding/gob"
	"log"
	"time"

	"github.com/eikeon/dynamodb"
	"github.com/eikeon/hu"
)

var DB dynamodb.DynamoDB

var tableName string = "RachaelFrame"

type FrameItem struct {
	Symbol string `db:"HASH"`
	GOB    []byte
}

func init() {
	gob.Register(hu.Boolean(true))
	gob.Register(hu.String(""))
	gob.Register(hu.Abstraction{})
	gob.Register(hu.Application{})
	gob.Register(hu.Tuple{})
	gob.Register(hu.Number{})
	gob.Register(hu.Symbol(""))

	DB = dynamodb.NewDynamoDB()

	table, err := DB.Register(tableName, (*FrameItem)(nil))
	if err != nil {
		log.Fatal(err)
	}
	pt := dynamodb.ProvisionedThroughput{ReadCapacityUnits: 10, WriteCapacityUnits: 10}
	if _, err := DB.CreateTable(table.TableName, table.AttributeDefinitions, table.KeySchema, pt, nil); err != nil {
		log.Println(err)
	}

	for {
		if description, err := DB.DescribeTable(tableName, nil); err != nil {
			log.Println(err)
		} else {
			log.Println(description.Table.TableStatus)
			if description.Table.TableStatus == "ACTIVE" {
				break
			}
		}
		time.Sleep(time.Second)
	}
}

type dbframe struct {
	//map[hu.Symbol]hu.Term
}

func (f dbframe) Define(variable hu.Symbol, value hu.Term) {
	//log.Printf("Define: %v - value: %#v\n", variable, value)
	// TODO: check if defined before just setting
	var network bytes.Buffer
	enc := gob.NewEncoder(&network)

	err := enc.Encode(&value)
	if err != nil {
		log.Println("encode:", err)
		//f[variable] = value
		return
	}
	fi := &FrameItem{Symbol: variable.String(), GOB: network.Bytes()}
	//log.Println("PUT:", fi)
	if _, err := DB.PutItem(tableName, DB.ToItem(fi), nil); err != nil {
		log.Print(err)
		return
	}
}

func (f dbframe) Set(variable hu.Symbol, value hu.Term) bool {
	_, ok := f.Get(variable)
	if ok {
		f.Define(variable, value)
	} else {
		panic(variable) //panic(hu.UnboundVariableError{variable, "set"})
	}
	return true
}

func (f dbframe) Get(variable hu.Symbol) (hu.Term, bool) {
	//value, ok := frame[variable]
	fi := &FrameItem{Symbol: variable.String()}
	if f, err := DB.GetItem(tableName, DB.ToKey(fi), nil); err != nil {
		log.Print(err)
	} else {
		//log.Println("f:", f)
		if f.Item != nil {
			i := DB.FromItem(tableName, *f.Item).(*FrameItem)
			//log.Println("Got:", i)
			dec := gob.NewDecoder(bytes.NewReader(i.GOB))
			var t hu.Term
			err := dec.Decode(&t)
			if err != nil {
				log.Fatal("decode:", err)
			}
			return t, true
		} else {
			//log.Println("Didn't find item")
		}
	}
	//value, ok := f[variable]
	//return value, ok
	return nil, false
}
